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

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/audio"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/playlist"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

const autoLeaveDuration = 3 * time.Minute

var (
	//ErrDifferentVoiceChannel = fmt.Errorf("voice connected to a different voice channel")
	ErrNullVoiceChannelID = fmt.Errorf("null voice channel id")
)

type ErrorHandler func(err error)

type StateHandler func(state State)

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
	Download(ctx context.Context, request *psong.Item) (string, error)
	Delete(path string) error
}

type PlaylistManager interface {
	Head() *psong.Item
	Remove(force bool)
	Add(item *psong.Item)
	Current() *psong.Item
	Queue() []psong.Item

	Loop(state bool)
	LoopToggle()
	Radio(state bool)
	RadioToggle()
	Shuffle(state bool)
	ShuffleToggle()

	State() playlist.State
}

type State struct {
	Current  psong.Item    `json:"current"`
	Position time.Duration `json:"position"`
	Length   time.Duration `json:"length"`
	Loop     bool          `json:"loop"`
	Radio    bool          `json:"radio"`
	Shuffle  bool          `json:"shuffle"`
	Queue    []psong.Item  `json:"queue"`
}

type service struct {
	audio      AudioService
	playlist   PlaylistManager
	downloader Downloader

	currentVoice    discord.ChannelID
	commands        chan *command
	autoLeaveTicker *time.Ticker

	errors        chan error
	errorHandlers chan ErrorHandler

	states        chan State
	stateHandlers chan StateHandler
	posMx         *sync.Mutex
	songPosition  audio.SongPosition

	ctx context.Context
}

func New(ctx context.Context, playlist PlaylistManager, downloader Downloader, stateTick time.Duration) *service {
	player := &service{
		ctx:        ctx,
		playlist:   playlist,
		downloader: downloader,

		commands:        make(chan *command),
		autoLeaveTicker: time.NewTicker(autoLeaveDuration),

		errors:        make(chan error),
		errorHandlers: make(chan ErrorHandler),

		states:        make(chan State),
		stateHandlers: make(chan StateHandler),
		posMx:         &sync.Mutex{},
	}

	go player.processCommands(ctx)
	go player.processOther(ctx, stateTick)
	go player.processErrors(ctx)
	go player.processStates(ctx)
	return player
}

func (s *service) SubscribeOnErrors(h ErrorHandler) {
	go func() {
		s.errorHandlers <- h
	}()
}

func (s *service) SubscribeOnStates(h StateHandler) {
	go func() {
		s.stateHandlers <- h
	}()
}

func (s *service) processCommands(ctx context.Context) {
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

func (s *service) processCommand(cmd *command, ctx context.Context, logger *zap.Logger) error {
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
		if err := s.downloader.Delete(cmd.source); err != nil {
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

func (s *service) play(ctx context.Context, voiceChannel discord.ChannelID) error {
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
	filePath, err := s.downloader.Download(ctx, song)
	if err != nil {
		// song is not available anymore, so we remove it from playlist
		s.playlist.Remove(true)
		return errors.Wrapf(err, "download song url %s", song.URL)
	}

	logger.Info("play song")
	s.audio.Play(ctx, filePath, 0)
	return nil
}

func (s *service) listenAudioInstance(ctx context.Context) {
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
			s.commands <- &command{typ: commandPlay}
			s.commands <- &command{typ: commandDeleteSong, source: source}

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

func (s *service) processOther(ctx context.Context, duration time.Duration) {
	t := time.NewTicker(duration)
	defer t.Stop()
	defer s.autoLeaveTicker.Stop()
	for {
		select {
		case <-t.C:
			s.commands <- &command{typ: commandSendState}
		case <-s.autoLeaveTicker.C:
			s.commands <- &command{typ: commandDisconnectIdle}
		case <-ctx.Done():
			return
		}
	}
}

func (s *service) error(logger *zap.Logger, err error) {
	if err == nil {
		return
	}
	logger.Error("failed to", zap.Error(err))
	go func() {
		s.errors <- err
	}()
}

func (s *service) processErrors(ctx context.Context) {
	handlers := make([]ErrorHandler, 0, 2)
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

func (s *service) processStates(ctx context.Context) {
	handlers := make([]StateHandler, 0, 2)
	defer close(s.stateHandlers)
	for {
		select {
		case state, ok := <-s.states:
			if !ok {
				return
			}
			for _, h := range handlers {
				go h(state)
			}
		case h := <-s.stateHandlers:
			handlers = append(handlers, h)
		case <-ctx.Done():
			return
		}
	}
}

func (s *service) sendState() {
	s.posMx.Lock()
	pos := s.songPosition.Elapsed
	length := s.songPosition.Length
	s.posMx.Unlock()

	queue := s.playlist.Queue()
	state := s.playlist.State()
	s.states <- State{
		Current:  queue[0],
		Position: pos,
		Length:   length,
		Loop:     state.Loop,
		Radio:    state.Radio,
		Shuffle:  state.Shuffle,
		Queue:    queue,
	}
}
