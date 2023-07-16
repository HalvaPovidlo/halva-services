package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/voice"
	"github.com/diamondburned/arikawa/v3/voice/voicegateway"
	"github.com/diamondburned/oggreader"
	"go.uber.org/zap"

	ds "github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/discord"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

type Service struct {
	session  *voice.Session
	workChan chan struct{}
	finished chan struct{}
	cancel   context.CancelFunc
}

func New(ctx context.Context, channelID discord.ChannelID) (*Service, error) {
	session, err := voice.NewSession(ds.State)
	if err != nil {
		return nil, fmt.Errorf("create a voice session: %w", err)
	}

	if err := session.JoinChannel(ctx, channelID, false, true); err != nil {
		return nil, fmt.Errorf("connect to voice channel: %w", err)
	}

	return &Service{
		session:  session,
		workChan: make(chan struct{}, 1),
		finished: make(chan struct{}),
	}, nil
}

func (s *Service) Play(ctx context.Context, source string, position time.Duration) bool {
	select {
	case s.workChan <- struct{}{}:
		go func() {
			defer func() {
				s.cancel = nil
				s.finished <- struct{}{}
				<-s.workChan
			}()
			ctx, cancel := context.WithCancel(ctx)
			logger := contexts.GetLogger(ctx)
			s.cancel = cancel

			ffmpeg, stdout, stderr, err := ffmpegStart(ctx, source, position)
			if err != nil {
				logger.Error("ffmpeg start", zap.Error(err))
				return
			}

			if err := s.session.Speaking(ctx, voicegateway.Microphone); err != nil {
				logger.Error("failed to send speaking packet to discord", zap.Error(err))
				return
			}

			if err := oggreader.DecodeBuffered(s.session, stdout); err != nil {
				logger.Error("failed to decode buffered ffmpeg stdout", zap.Error(err))
				return
			}

			if err, std := ffmpeg.Wait(), stderr.String(); err != nil && std != "" {
				logger.Error("ffmpeg finished", zap.String("stderr", std), zap.Error(err))
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

func (s *Service) Finished() <-chan struct{} {
	return s.finished
}

func (s *Service) DestroyIdle() {
	select {
	case s.workChan <- struct{}{}:
		s.session.Leave(context.Background())
		close(s.finished)
		close(s.workChan)
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

func ffmpegStart(ctx context.Context, source string, position time.Duration) (*exec.Cmd, io.ReadCloser, bytes.Buffer, error) {
	ffmpeg := exec.CommandContext(ctx, "ffmpeg",
		"-loglevel", "error", "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5",
		"-ss", formatTime(position),
		"-i", source,
		"-vn", "-codec", "libopus", "-vbr", "off", "-frame_duration", "20", "-f", "opus", "-")

	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return nil, nil, bytes.Buffer{}, fmt.Errorf("get ffmpeg stdout: %w", err)

	}

	var stderr bytes.Buffer
	ffmpeg.Stderr = &stderr
	if err := ffmpeg.Start(); err != nil {
		return nil, nil, bytes.Buffer{}, fmt.Errorf("start ffmpeg process: %w", err)
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
