package player

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"

	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player/audio"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

const autoLeaveDuration = 2 * time.Minute

type AudioService interface {
	Play(ctx context.Context, source string, position time.Duration) bool
	Stop()
	Destroy()
	DestroyIdle()
	Finished() <-chan struct{}
}

type Downloader interface {
	Download(*psong.Item) (*psong.Item, error)
}

type Searcher interface {
	Find(request *SongRequest) (*psong.Item, error)
	Radio() (*psong.Item, error)
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
	current psong.Item
	loop    bool
	radio   bool
	shuffle bool
	queue   []psong.Item
}

type Service struct {
	audio      AudioService
	playlist   PlaylistManager
	downloader Downloader
	searcher   Searcher

	state           *State
	commands        chan *Command
	states          chan *State
	autoLeaveTicker *time.Ticker

	ctx context.Context
}

func New(ctx context.Context, stateTick time.Duration) *Service {
	player := &Service{
		ctx:             ctx,
		autoLeaveTicker: time.NewTicker(autoLeaveDuration),
	}

	go player.processCommands(ctx)
	go player.sendStates(ctx, stateTick)
	return player
}

func (s *Service) Input() chan<- *Command {
	return s.commands
}

func (s *Service) Status() <-chan *State {
	return s.states
}

func (s *Service) processCommands(ctx context.Context) {
	s.commands = make(chan *Command)
	defer close(s.commands)

	for {
		select {
		case cmd := <-s.commands:
			switch cmd.t {
			case commandPlay:
				if err := s.play(ctx, cmd.voiceChannel); err != nil {
					fmt.Println(err)
				}
			case commandSkip:
				s.playlist.LoopDisable()
				if s.audio != nil {
					s.audio.Stop()
				}
			case commandEnqueue:
				song, err := s.searcher.Find(cmd.song)
				if err != nil {
					fmt.Println(err)
				}
				s.playlist.Add(song)
			case commandRadio:
				s.state.radio = true
			case commandRadioOff:
				s.state.radio = false
			case commandLoop:
				s.state.loop = true
				s.playlist.Loop()
			case commandLoopOff:
				s.state.loop = false
				s.playlist.LoopDisable()
			case commandShuffle:
				s.state.shuffle = true
				s.playlist.Shuffle()
			case commandShuffleOff:
				s.state.shuffle = false
				s.playlist.ShuffleDisable()
			case commandSendState:
				copy(s.state.queue, s.playlist.Queue())
				go func() { s.states <- s.state }()
			case commandDisconnectIdle:
				s.audio.DestroyIdle()
			case commandDisconnect:
				s.audio.Destroy()
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) play(ctx context.Context, voiceChannel discord.ChannelID) error {
	var err error
	if s.audio == nil {
		if voiceChannel != discord.NullChannelID {
			return nil
		}
		s.audio, err = audio.New(ctx, voiceChannel)
		if err != nil {
			return fmt.Errorf("create new audio session: %w", err)
		}
		go s.listenFinished(s.ctx)
	}

	song := s.playlist.Peek()
	if song == nil {
		if s.state.radio {
			song, err = s.searcher.Radio()
			if err != nil {
				return fmt.Errorf("get radio song: %w", err)
			}
		}
		return nil
	}

	song, err = s.downloader.Download(song)
	if err != nil {
		return fmt.Errorf("download song: %w", err)
	}

	if s.audio.Play(ctx, song.FilePath, 0) {
		s.state.current = *song
		s.playlist.Remove()
	}

	return nil
}

func (s *Service) listenFinished(ctx context.Context) {
	finished := s.audio.Finished()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-finished:
			if !ok {
				return
			}
			s.autoLeaveTicker.Reset(autoLeaveDuration)
			s.commands <- &Command{t: commandPlay}
		}
	}
}

func (s *Service) sendStates(ctx context.Context, duration time.Duration) {
	t := time.NewTicker(duration)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.commands <- &Command{t: commandSendState}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) autoLeave(ctx context.Context) {
	defer s.autoLeaveTicker.Stop()
	for {
		select {
		case <-s.autoLeaveTicker.C:
			s.commands <- &Command{t: commandDisconnectIdle}
		case <-ctx.Done():
			return
		}
	}
}
