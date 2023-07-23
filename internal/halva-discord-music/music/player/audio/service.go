package audio

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/voice"
	"github.com/diamondburned/arikawa/v3/voice/udp"
	"github.com/diamondburned/arikawa/v3/voice/voicegateway"
	"github.com/diamondburned/oggreader"
	"go.uber.org/zap"

	ds "github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/discord"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

const (
	frameDuration = 60 // ms
	timeIncrement = 2880
)

type SongPosition struct {
	Elapsed time.Duration
	Length  time.Duration
}

type Service struct {
	session   *voice.Session
	length    time.Duration
	workChan  chan struct{}
	finished  chan string
	songTicks chan SongPosition
	cancel    context.CancelFunc
}

func New(ctx context.Context, channelID discord.ChannelID) (*Service, error) {
	session, err := voice.NewSession(ds.State)
	if err != nil {
		return nil, fmt.Errorf("create a voice session: %+w", err)
	}

	session.SetUDPDialer(udp.DialFuncWithFrequency(
		frameDuration*time.Millisecond,
		timeIncrement,
	))

	if err := session.JoinChannel(ctx, channelID, false, true); err != nil {
		return nil, fmt.Errorf("connect to voice channel: %+w", err)
	}

	return &Service{
		session:   session,
		workChan:  make(chan struct{}, 1),
		finished:  make(chan string),
		songTicks: make(chan SongPosition),
	}, nil
}

func (s *Service) Play(ctx context.Context, source string, position time.Duration) bool {
	select {
	case s.workChan <- struct{}{}:
		go func() {
			defer func() {
				s.cancel = nil
				s.length = 0
				s.finished <- source
				<-s.workChan
			}()
			ctx, cancel := context.WithCancel(ctx)
			logger := contexts.GetLogger(ctx).With(zap.String("source", source))
			s.cancel = cancel

			length, err := getAudioLength(source)
			if err != nil {
				logger.Error("ffmpeg get audio length", zap.Error(err))
				return
			}
			s.length = length

			ffmpeg, stdout, stderr, err := ffmpegStart(ctx, source, position)
			if err != nil {
				logger.Error("ffmpeg start", zap.Error(err))
				return
			}

			go s.streamSongPosition(stderr)

			if err := s.session.Speaking(ctx, voicegateway.Microphone); err != nil {
				logger.Error("failed to send speaking packet to discord", zap.Error(err))
				return
			}

			if err := oggreader.DecodeBuffered(s.session, stdout); err != nil {
				logger.Error("failed to decode buffered ffmpeg stdout", zap.Error(err))
				return
			}

			if err, ctxErr := ffmpeg.Wait(), ctx.Err(); err != nil && ctxErr != context.Canceled {
				logger.Error("ffmpeg finished", zap.Error(err))
			}
		}()
	default:
		return false
	}
	return true
}

func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Service) Finished() <-chan string {
	return s.finished
}

func (s *Service) DestroyIdle() bool {
	select {
	case s.workChan <- struct{}{}:
		s.session.Leave(context.Background())
		close(s.finished)
		close(s.workChan)
		return true
	default:
		return false
	}
}

func (s *Service) Idle() bool {
	select {
	case s.workChan <- struct{}{}:
		<-s.workChan
		return true
	default:
		return false
	}
}

func (s *Service) Destroy() {
	s.cancel()
	for {
		select {
		case s.workChan <- struct{}{}:
			s.session.Leave(context.Background())
			close(s.finished)
			close(s.workChan)
			return
		}
	}
}

func (s *Service) SongPosition() <-chan SongPosition {
	return s.songTicks
}

func ffmpegStart(ctx context.Context, source string, position time.Duration) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if position == 0 {
		args = append(args, "-re") // high cpu, impossible to start from not 0 position
	}
	args = append(args, []string{
		"-threads", "1",
		"-i", source,
		"-ss", formatTime(position),
		"-c:a", "libopus",
		"-b:a", "96k",
		"-frame_duration", strconv.Itoa(frameDuration),
		"-vbr", "off",
		"-f", "opus",
		"-progress", "pipe:2",
		"-",
	}...)

	ffmpeg := exec.CommandContext(ctx, "ffmpeg", args...)

	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get ffmpeg stdout: %+w", err)

	}
	stderr, err := ffmpeg.StderrPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get ffmpeg stderr: %+w", err)

	}

	if err := ffmpeg.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("start ffmpeg process: %+w", err)
	}
	return ffmpeg, stdout, stderr, nil
}

func formatTime(duration time.Duration) string {
	totalSeconds := int64(duration.Seconds())
	days := totalSeconds / 86400
	hours := totalSeconds % 86400 / 3600
	minutes := totalSeconds % 3600 / 60
	seconds := totalSeconds % 60

	if days > 0 {
		return fmt.Sprintf("%02d:%02d:%02d:%02d", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (s *Service) streamSongPosition(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time_ms=") {
			microsecondsStr := strings.TrimPrefix(line, "out_time_ms=")
			microseconds, err := strconv.ParseInt(microsecondsStr, 10, 64)
			if err != nil {
				continue
			}

			s.songTicks <- SongPosition{
				Elapsed: time.Duration(microseconds) * time.Microsecond,
				Length:  s.length,
			}
		}
	}
}

func getAudioLength(path string) (time.Duration, error) {
	out, _ := exec.Command("ffmpeg", "-i", path).CombinedOutput()

	re := regexp.MustCompile(`Duration: (.*?),`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not find duration in ffmpeg output")
	}

	t, err := time.Parse("15:04:05", matches[1])
	if err != nil {
		return 0, err
	}
	duration := time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second
	return duration, nil
}

// streaming ffmpeg
//ffmpeg := exec.CommandContext(ctx, "ffmpeg",
//	"-loglevel", "error", "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5",
//	"-ss", formatTime(position),
//	"-i", source,
//	"-vn", "-codec", "libopus", "-vbr", "off", "-frame_duration", "20", "-f", "opus", "-")
