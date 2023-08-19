package player

import (
	"github.com/diamondburned/arikawa/v3/discord"

	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

func (s *service) Play(item *psong.Item, userID discord.UserID, voiceID discord.ChannelID, traceID string) {
	s.playlist.Add(item)
	s.commands <- play(userID, voiceID, traceID)
}

func (s *service) Skip(userID discord.UserID, voiceID discord.ChannelID, traceID string) {
	s.commands <- skip(userID, voiceID, traceID)
}

func (s *service) Disconnect(userID discord.UserID, voiceID discord.ChannelID, traceID string) {
	s.commands <- disconnect(userID, voiceID, traceID)
}

func (s *service) Loop(state bool) {
	s.playlist.Loop(state)
}

func (s *service) LoopToggle() {
	s.playlist.LoopToggle()
}

func (s *service) Radio(state bool) {
	s.playlist.Radio(state)
}

func (s *service) RadioToggle() {
	s.playlist.RadioToggle()
}

func (s *service) Shuffle(state bool) {
	s.playlist.Shuffle(state)
}

func (s *service) ShuffleToggle() {
	s.playlist.ShuffleToggle()
}
