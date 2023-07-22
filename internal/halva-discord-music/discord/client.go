package discord

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/voice"
)

const HalvaGuildID = discord.GuildID(623964456929591297)

var (
	State *state.State
	Self  *discord.User
	App   *discord.Application
)

func NewClient(token string) {
	State = state.NewWithIntents("Bot "+token,
		gateway.IntentGuilds,
		gateway.IntentGuildMessages,
		gateway.IntentGuildVoiceStates,
		gateway.IntentDirectMessages)
	voice.AddIntents(State)
	return
}

func Connect(ctx context.Context) error {
	err := State.Open(ctx)

	if err == nil {
		Self, err = State.Me()
	}
	if err == nil {
		App, err = State.CurrentApplication()
	}

	return err
}

func Close() {
	State.Close()
}
