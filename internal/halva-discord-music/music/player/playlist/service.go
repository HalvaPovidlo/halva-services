package playlist

import (
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

type service struct {
	queue   []psong.Item
	loop    bool
	shuffle bool
}

func New() *service {
	return &service{
		queue: make([]psong.Item, 0, 25),
	}
}

func (s *service) Add(item *psong.Item) {
	s.queue = append(s.queue, *item)
}

func (s *service) Peek() *psong.Item {
	if len(s.queue) == 0 {
		return nil
	}
	song := s.queue[0]
	return &song
}

func (s *service) Queue() []psong.Item {
	queue := make([]psong.Item, len(s.queue))
	copy(queue, s.queue)
	return queue
}

func (s *service) Remove() {
	if s.loop || len(s.queue) == 0 {
		return
	}
	s.queue = s.queue[1:]
}

func (s *service) Loop() {
	s.loop = true
}

func (s *service) LoopDisable() {
	s.loop = false
}

func (s *service) Shuffle() {
	s.shuffle = true
}

func (s *service) ShuffleDisable() {
	s.shuffle = false
}
