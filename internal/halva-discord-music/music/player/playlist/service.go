package playlist

import (
	"math/rand"
	"sync"

	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

type radioService interface {
	Radio(minPlaybacks int64) (*psong.Item, error)
}

type service struct {
	queue   []psong.Item
	radio   bool
	loop    bool
	shuffle bool

	radioService radioService
	mx           *sync.Mutex
}

func New(radioService radioService) *service {
	return &service{
		radioService: radioService,
		mx:           &sync.Mutex{},
		queue:        make([]psong.Item, 0, 25),
	}
}

func (s *service) Add(item *psong.Item) {
	s.mx.Lock()
	s.queue = append(s.queue, *item)
	s.mx.Unlock()
}

func (s *service) Head() *psong.Item {
	s.mx.Lock()
	defer s.mx.Unlock()

	if len(s.queue) == 0 {
		if s.radio {
			if r, err := s.radioService.Radio(3); err == nil {
				s.queue = append(s.queue, *r)
				return &s.queue[0]
			}
		}
		return nil
	}

	if s.loop {
		return &s.queue[0]
	}

	if s.shuffle {
		r := rand.Intn(len(s.queue))
		q := make([]psong.Item, 0, len(s.queue))
		q = append(q, s.queue[r])
		q = append(q, s.queue[:r]...)
		q = append(q, s.queue[r+1:]...)
		s.queue = q
	}

	return &s.queue[0]
}

func (s *service) Remove(force bool) {
	s.mx.Lock()
	if (s.loop && !force) || len(s.queue) == 0 {
		s.mx.Unlock()
		return
	}
	s.queue = s.queue[1:]
	s.mx.Unlock()
}

func (s *service) Current() *psong.Item {
	s.mx.Lock()
	defer s.mx.Unlock()

	if len(s.queue) == 0 {
		return nil
	}

	return &s.queue[0]
}

func (s *service) Queue() []psong.Item {
	s.mx.Lock()
	queue := make([]psong.Item, len(s.queue))
	copy(queue, s.queue)
	s.mx.Unlock()

	return queue
}

func (s *service) Loop(state bool) {
	s.mx.Lock()
	s.loop = state
	s.mx.Unlock()
}

func (s *service) LoopToggle() bool {
	s.mx.Lock()
	s.loop = !s.loop
	loop := s.loop
	s.mx.Unlock()
	return loop
}

func (s *service) Radio(state bool) {
	s.mx.Lock()
	s.radio = state
	s.mx.Unlock()
}

func (s *service) RadioToggle() bool {
	s.mx.Lock()
	s.radio = !s.radio
	radio := s.radio
	s.mx.Unlock()
	return radio
}

func (s *service) Shuffle(state bool) {
	s.mx.Lock()
	s.shuffle = state
	s.mx.Unlock()
}

func (s *service) ShuffleToggle() bool {
	s.mx.Lock()
	s.shuffle = !s.shuffle
	shuffle := s.shuffle
	s.mx.Unlock()
	return shuffle
}

type State struct {
	Loop    bool
	Radio   bool
	Shuffle bool
}

func (s *service) State() State {
	s.mx.Lock()
	defer s.mx.Unlock()

	return State{
		Loop:    s.loop,
		Radio:   s.radio,
		Shuffle: s.shuffle,
	}
}
