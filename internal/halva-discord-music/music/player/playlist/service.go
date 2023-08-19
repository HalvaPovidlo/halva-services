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
	*sync.Mutex
}

func New(radioService radioService) *service {
	return &service{
		radioService: radioService,
		queue:        make([]psong.Item, 0, 25),
	}
}

func (s *service) Add(item *psong.Item) {
	s.Lock()
	s.queue = append(s.queue, *item)
	s.Unlock()
}

func (s *service) Head() *psong.Item {
	s.Lock()
	defer s.Unlock()

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
	s.Lock()
	if (s.loop && !force) || len(s.queue) == 0 {
		s.Unlock()
		return
	}
	s.queue = s.queue[1:]
	s.Unlock()
}

func (s *service) Current() *psong.Item {
	s.Lock()
	defer s.Unlock()

	if len(s.queue) == 0 {
		return nil
	}

	return &s.queue[0]
}

func (s *service) Queue() []psong.Item {
	s.Lock()
	queue := make([]psong.Item, len(s.queue))
	copy(queue, s.queue)
	s.Unlock()

	return queue
}

func (s *service) Loop(state bool) {
	s.Lock()
	s.loop = state
	s.Unlock()
}

func (s *service) LoopToggle() {
	s.Lock()
	s.loop = !s.loop
	s.Unlock()
}

func (s *service) Radio(state bool) {
	s.Lock()
	s.radio = state
	s.Unlock()
}

func (s *service) RadioToggle() {
	s.Lock()
	s.radio = !s.radio
	s.Unlock()
}

func (s *service) Shuffle(state bool) {
	s.Lock()
	s.shuffle = state
	s.Unlock()
}

func (s *service) ShuffleToggle() {
	s.Lock()
	s.shuffle = !s.shuffle
	s.Unlock()
}

type State struct {
	Loop    bool
	Radio   bool
	Shuffle bool
}

func (s *service) State() State {
	s.Lock()
	defer s.Unlock()

	return State{
		Loop:    s.loop,
		Radio:   s.radio,
		Shuffle: s.shuffle,
	}
}
