// TODO: если бот отключился, то кидать событие destroy
package player

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/audio"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/search"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

// const autoLeaveDuration = 2 * time.Minute
const autoLeaveDuration = 15 * time.Second

type ErrorHandler func(err error)

type AudioService interface {
	Play(ctx context.Context, source string, position time.Duration) bool
	Stop()
	Destroy()
	DestroyIdle() bool
	Finished() <-chan string
	SongPosition() <-chan time.Duration
}

type Downloader interface {
	Download(url string) (*psong.Item, error)
	Delete(path string) error
}

type Searcher interface {
	Search(ctx context.Context, request *search.SongRequest) (*psong.Item, error)
	Radio(ctx context.Context) (*psong.Item, error)
}

type PlaylistManager interface {
	Add(item *psong.Item)
	Peek() *psong.Item
	Queue() []psong.Item
	Remove()
	Loop()
	LoopDisable()
	Shuffle()
	ShuffleDisable()
}

type State struct {
	Current  psong.Item    `json:"current"`
	Position time.Duration `json:"position"`
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
	commands        chan *playerCommand
	states          chan State
	autoLeaveTicker *time.Ticker

	errors        chan error
	errorHandlers chan ErrorHandler

	posMx        *sync.Mutex
	songPosition time.Duration

	ctx context.Context
}

func New(ctx context.Context, playlist PlaylistManager, downloader Downloader, searcher Searcher, stateTick time.Duration) *Service {
	player := &Service{
		ctx:        ctx,
		playlist:   playlist,
		downloader: downloader,
		searcher:   searcher,

		commands:        make(chan *playerCommand),
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

func (s *Service) Input() chan<- *playerCommand {
	return s.commands
}

func (s *Service) Status() <-chan State {
	return s.states
}

func (s *Service) processCommands(ctx context.Context) {
	defer close(s.commands)
	for {
		select {
		case cmd := <-s.commands:
			ctx, logger := cmd.ContextLogger(ctx)
			// ignore spam
			if !(cmd.t == commandSendState || (cmd.t == commandDisconnectIdle && s.audio == nil)) {
				logger.Info("process command")
			}
			switch cmd.t {
			case commandPlay:
				s.error(logger, s.play(ctx, cmd.voiceChannel))
			case commandSkip:
				s.playlist.LoopDisable()
				if s.audio != nil {
					s.audio.Stop()
				}
			case commandEnqueue:
				song, err := s.searcher.Search(ctx, cmd.songRequest)
				if err != nil {
					s.error(logger, err)
					break
				}
				s.playlist.Add(song)
			case commandRadio:
				s.state.Radio = true
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
			case commandDeleteSong:
				s.error(logger, s.downloader.Delete(cmd.path))
			case commandSendState:
				s.posMx.Lock()
				s.state.Position = s.songPosition
				s.posMx.Unlock()

				s.state.Queue = s.playlist.Queue()
				go func() { s.states <- s.state }()
			case commandDisconnectIdle:
				if s.audio != nil && s.audio.DestroyIdle() {
					s.audio = nil
				}
			case commandDisconnect:
				if s.audio != nil {
					s.audio.Destroy()
					s.audio = nil
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) play(ctx context.Context, voiceChannel discord.ChannelID) error {
	var err error
	if s.audio == nil {
		if voiceChannel == discord.NullChannelID {
			return fmt.Errorf("null channel id")
		}
		s.audio, err = audio.New(ctx, voiceChannel)
		if err != nil {
			return fmt.Errorf("create new audio session: %+w", err)
		}
		go s.listenAudioInstance(s.ctx)
	}

	song := s.playlist.Peek()
	if song == nil {
		if s.state.Radio {
			song, err = s.searcher.Radio(ctx)
			if err != nil {
				return fmt.Errorf("get radio song: %+w", err)
			}
		}
		return nil
	}

	song, err = s.downloader.Download(song.URL)
	if err != nil {
		return fmt.Errorf("download song: %+w", err)
	}

	if s.audio.Play(ctx, song.FilePath, 0) {
		s.state.Current = *song
		s.playlist.Remove()
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
			s.commands <- &playerCommand{t: commandPlay}
			s.commands <- &playerCommand{t: commandDeleteSong, path: source}

			s.posMx.Lock()
			s.songPosition = 0
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
			s.commands <- &playerCommand{t: commandSendState}
		case <-s.autoLeaveTicker.C:
			s.commands <- &playerCommand{t: commandDisconnectIdle}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) SubscribeOnErrors(h ErrorHandler) {
	go func() {
		s.errorHandlers <- h
	}()
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
