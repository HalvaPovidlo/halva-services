package player

import "github.com/diamondburned/arikawa/v3/discord"

const (
	commandPlay = iota
	commandSkip
	commandEnqueue
	commandLoop
	commandLoopOff
	commandRadio
	commandRadioOff
	commandShuffle
	commandShuffleOff
	commandSendState
	commandDisconnectIdle
	commandDisconnect
)

type SongRequest struct {
	ID   string
	URL  string
	Text string
}

type Command struct {
	t            int
	song         *SongRequest
	voiceChannel discord.ChannelID
}
