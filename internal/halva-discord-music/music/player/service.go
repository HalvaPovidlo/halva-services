// TODO: если бот отключился, то кидать событие destroy
package player

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/download"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/audio"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/search"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

const autoLeaveDuration = 2 * time.Minute

var (
	ErrDifferentVoiceChannel = fmt.Errorf("voice connected to a different voice channel")
	ErrNullVoiceChannelID    = fmt.Errorf("null voice channel id")
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
	Radio(minPlaybacks int64) (*psong.Item, error)
}

type PlaylistManager interface {
	Add(item *psong.Item)
	Peek() *psong.Item
	Queue() []psong.Item
	Remove()
	RemoveForce()
	Loop()
	LoopDisable()
	Shuffle()
	ShuffleDisable()
}

type State struct {
	Current  psong.Item    `json:"current"`
	Position time.Duration `json:"position"`
	Length   time.Duration `json:"length"`
	Loop     bool          `json:"loop"`
	Radio    bool          `json:"radio"`
	//Shuffle  bool          `json:"shuffle"`
	Queue []psong.Item `json:"queue"`
}

type Service struct {
	audio      AudioService
	playlist   PlaylistManager
	downloader Downloader
	searcher   Searcher

	state           State
	currentVoice    discord.ChannelID
	commands        chan *Command
	states          chan State
	autoLeaveTicker *time.Ticker

	errors        chan error
	errorHandlers chan ErrorHandler

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
	handled, err := s.processPrivateCommand(cmd, ctx, logger)
	if err != nil || handled {
		return err
	}

	handled, err = s.processPublicCommand(cmd, ctx, logger)
	if err != nil || handled {
		return err
	}

	return fmt.Errorf("unknown command: %s", cmd.typ)
}

func (s *Service) processPrivateCommand(cmd *Command, ctx context.Context, logger *zap.Logger) (bool, error) {
	switch cmd.typ {
	case commandPlay:
		return true, s.play(ctx, cmd.voiceChannelID)
	case commandRemove:
		s.playlist.Remove()
	case commandDeleteSong:
		if err := s.downloader.Delete(cmd.downloadRequest); err != nil {
			return false, fmt.Errorf("delete song")
		}
	case commandSendState:
		s.sendState()
	case commandDisconnectIdle:
		if s.audio != nil && s.audio.DestroyIdle() {
			logger.Info("process command")
			s.audio = nil
			s.currentVoice = discord.NullChannelID
		}
	case commandDisconnect:
		if s.audio != nil {
			s.audio.Destroy()
			s.audio = nil
			s.currentVoice = discord.NullChannelID
		}
	default:
		return false, nil
	}

	return true, nil
}

func (s *Service) processPublicCommand(cmd *Command, ctx context.Context, logger *zap.Logger) (bool, error) {
	if s.audio != nil && cmd.voiceChannelID != s.currentVoice {
		return false, ErrDifferentVoiceChannel
	}

	logger.Info("process command")
	switch cmd.typ {
	case commandSkip:
		if s.audio != nil {
			s.playlist.RemoveForce()
			s.audio.Stop()
		}
	case commandEnqueue:
		song, err := s.searcher.Search(ctx, cmd.searchRequest)
		if err != nil {
			return false, fmt.Errorf("search song %s: %+w", cmd.searchRequest.Text, err)
		}
		s.playlist.Add(song)

		s.sendPlayIfNotConnected(cmd)
	case commandRadio:
		s.state.Radio = true
		s.sendPlayIfNotConnected(cmd)
	case commandRadioOff:
		s.state.Radio = false
	case commandLoop:
		s.state.Loop = true
		s.playlist.Loop()
	case commandLoopOff:
		s.state.Loop = false
		s.playlist.LoopDisable()
	case commandShuffle:
		//s.state.Shuffle = true
		s.playlist.Shuffle()
	case commandShuffleOff:
		//s.state.Shuffle = false
		s.playlist.ShuffleDisable()
	case commandDisconnect:
		if s.audio != nil {
			s.audio.Destroy()
			s.audio = nil
			s.currentVoice = discord.NullChannelID
		}
	default:
		return false, nil
	}

	return true, nil
}

func (s *Service) play(ctx context.Context, voiceChannel discord.ChannelID) error {
	var err error
	if s.audio == nil {
		if voiceChannel == discord.NullChannelID {
			return ErrNullVoiceChannelID
		}
		s.audio, err = audio.New(ctx, voiceChannel)
		if err != nil {
			return fmt.Errorf("create new audio session: %+w", err)
		}
		s.currentVoice = voiceChannel
		go s.listenAudioInstance(s.ctx)
	}

	if !s.audio.Idle() {
		return nil
	}

	song := s.playlist.Peek()
	if song == nil {
		if s.state.Radio {
			song, err = s.searcher.Radio(3)
			if err != nil {
				s.state.Radio = false
				return fmt.Errorf("get radio song: %+w", err)
			}
		} else {
			s.state.Current = psong.Item{}
			return nil
		}
	}

	logger := contexts.GetLogger(ctx)
	logger.Debug("download song", zap.String("url", song.URL))
	song.FilePath, err = s.downloader.Download(ctx, &download.Request{
		ID:      string(song.ID),
		Source:  song.URL,
		Service: psong.ServiceType(song.Service),
	})
	if err != nil {
		s.playlist.RemoveForce()
		return fmt.Errorf("download song, %s: %+w", song.URL, err)
	}

	logger.Info("play song", zap.String("title", song.Title))
	if s.audio.Play(ctx, song.FilePath, 0) {
		s.state.Current = *song
		//s.playlist.Remove()
	}

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
			s.commands <- &Command{typ: commandRemove}
			s.commands <- &Command{typ: commandPlay}
			s.commands <- &Command{typ: commandDeleteSong, downloadRequest: &download.Request{
				Source: source,
			}}

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

func (s *Service) sendPlayIfNotConnected(cmd *Command) {
	if s.audio == nil && cmd.voiceChannelID != discord.NullChannelID {
		cmd := *cmd
		cmd.typ = commandPlay
		go func() {
			s.commands <- &cmd
		}()
	}
}

func (s *Service) sendState() {
	s.posMx.Lock()
	s.state.Position = s.songPosition.Elapsed
	s.state.Length = s.songPosition.Length
	s.posMx.Unlock()

	s.state.Queue = s.playlist.Queue()
	s.states <- s.state
	//go func() { s.states <- s.state }()
}
