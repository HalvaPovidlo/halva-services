// TODO: если бот отключился, то кидать событие destroy
// TODO: stop downloader if skip current song +++ вынести загрузку в отедльный поток
// можно сделать такую же схему как и с аудио с workChan и если загрузка не началась, то скипать событие
package player

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/download"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/audio"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/search"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

const autoLeaveDuration = 3 * time.Minute

var (
	//ErrDifferentVoiceChannel = fmt.Errorf("voice connected to a different voice channel")
	ErrNullVoiceChannelID = fmt.Errorf("null voice channel id")
)

type ErrorHandler func(err error)

type AudioService interface {
	Play(ctx context.Context, source string, position time.Duration) bool
	Stop()
	Destroy()
	DestroyIdle() bool
	Idle() bool
	Finished() <-chan string
	SongPosition() <-chan audio.SongPosition
}

type Downloader interface {
	Download(ctx context.Context, r *download.Request) (string, error)
	Delete(r *download.Request) error
}

type Searcher interface {
	Search(ctx context.Context, request *search.Request) (*psong.Item, error)
}

type PlaylistManager interface {
	Head() *psong.Item
	Remove(force bool)
}

type State struct {
	Position time.Duration `json:"position"`
	Length   time.Duration `json:"length"`
}

type Service struct {
	audio      AudioService
	playlist   PlaylistManager
	downloader Downloader
	searcher   Searcher

	currentVoice    discord.ChannelID
	commands        chan *Command
	autoLeaveTicker *time.Ticker

	errors        chan error
	errorHandlers chan ErrorHandler

	// TODO: simplify state logic
	state        State
	states       chan State
	posMx        *sync.Mutex
	songPosition audio.SongPosition

	ctx context.Context
}

func New(ctx context.Context, playlist PlaylistManager, downloader Downloader, searcher Searcher, stateTick time.Duration) *Service {
	player := &Service{
		ctx:        ctx,
		playlist:   playlist,
		downloader: downloader,
		searcher:   searcher,

		commands:        make(chan *Command),
		states:          make(chan State),
		autoLeaveTicker: time.NewTicker(autoLeaveDuration),

		errors:        make(chan error),
		errorHandlers: make(chan ErrorHandler),

		posMx: &sync.Mutex{},
	}

	go player.processCommands(ctx)
	go player.processOther(ctx, stateTick)
	go player.processErrors(ctx)
	return player
}

func (s *Service) Input() chan<- *Command {
	return s.commands
}

func (s *Service) Status() <-chan State {
	return s.states
}

func (s *Service) SubscribeOnErrors(h ErrorHandler) {
	go func() {
		s.errorHandlers <- h
	}()
}

func (s *Service) processCommands(ctx context.Context) {
	defer close(s.commands)
	for {
		select {
		case cmd := <-s.commands:
			ctx, logger := cmd.contextLogger(ctx)
			if err := s.processCommand(cmd, ctx, logger); err != nil {
				s.error(logger, err)
				if cmd.typ == commandPlay && !errors.Is(err, ErrNullVoiceChannelID) {
					cmd := cmd
					go func() { s.commands <- cmd }()
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) processCommand(cmd *Command, ctx context.Context, logger *zap.Logger) error {
	switch cmd.typ {
	case commandPlay:
		return s.play(ctx, cmd.voiceChannelID)
	case commandSkip:
		if s.audio != nil {
			s.audio.Stop()
		}
	case commandDisconnect:
		if s.audio != nil {
			s.audio.Destroy()
			s.audio = nil
			s.currentVoice = discord.NullChannelID
		}
	case commandDeleteSong:
		if err := s.downloader.Delete(cmd.downloadRequest); err != nil {
			return errors.Wrap(err, "delete song")
		}
	case commandSendState:
		s.sendState()
	case commandDisconnectIdle:
		if s.audio != nil && s.audio.DestroyIdle() {
			logger.Info("process command")
			s.audio = nil
			s.currentVoice = discord.NullChannelID
		}
	}

	return nil
}

func (s *Service) play(ctx context.Context, voiceChannel discord.ChannelID) error {
	var err error
	if s.audio == nil {
		if voiceChannel == discord.NullChannelID {
			return ErrNullVoiceChannelID
		}
		s.audio, err = audio.New(ctx, voiceChannel)
		if err != nil {
			return errors.Wrap(err, "create new audio session")
		}
		s.currentVoice = voiceChannel
		go s.listenAudioInstance(s.ctx)
	}

	if !s.audio.Idle() {
		return nil
	}

	song := s.playlist.Head()
	if song == nil {
		return nil
	}

	logger := contexts.GetLogger(ctx).With(zap.String("url", song.URL), zap.String("title", song.Title))
	logger.Info("download song")
	song.FilePath, err = s.downloader.Download(ctx, &download.Request{
		ID:      string(song.ID),
		Source:  song.URL,
		Service: psong.ServiceType(song.Service),
	})
	if err != nil {
		// song is not available anymore, so we remove it from playlist
		s.playlist.Remove(true)
		return errors.Wrapf(err, "download song url %s", song.URL)
	}

	logger.Info("play song")
	s.audio.Play(ctx, song.FilePath, 0)
	return nil
}

func (s *Service) listenAudioInstance(ctx context.Context) {
	finished := s.audio.Finished()
	ticks := s.audio.SongPosition()
	for {
		select {
		case source, ok := <-finished:
			if !ok {
				return
			}
			s.autoLeaveTicker.Reset(autoLeaveDuration)
			s.playlist.Remove(false)
			s.commands <- &Command{typ: commandPlay}
			s.commands <- &Command{typ: commandDeleteSong, downloadRequest: &download.Request{Source: source}}

			s.posMx.Lock()
			s.songPosition.Length = 0
			s.songPosition.Elapsed = 0
			s.posMx.Unlock()

		case pos := <-ticks:
			s.posMx.Lock()
			s.songPosition = pos
			s.posMx.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) processOther(ctx context.Context, duration time.Duration) {
	t := time.NewTicker(duration)
	defer t.Stop()
	defer s.autoLeaveTicker.Stop()
	for {
		select {
		case <-t.C:
			s.commands <- &Command{typ: commandSendState}
		case <-s.autoLeaveTicker.C:
			s.commands <- &Command{typ: commandDisconnectIdle}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) error(logger *zap.Logger, err error) {
	if err == nil {
		return
	}
	logger.Error("failed to", zap.Error(err))
	go func() {
		s.errors <- err
	}()
}

func (s *Service) processErrors(ctx context.Context) {
	handlers := make([]ErrorHandler, 0)
	defer close(s.errorHandlers)
	for {
		select {
		case err, ok := <-s.errors:
			if !ok {
				return
			}
			for _, h := range handlers {
				go h(err)
			}
		case h := <-s.errorHandlers:
			handlers = append(handlers, h)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) sendState() {
	s.posMx.Lock()
	s.state.Position = s.songPosition.Elapsed
	s.state.Length = s.songPosition.Length
	s.posMx.Unlock()

	s.states <- s.state
}
