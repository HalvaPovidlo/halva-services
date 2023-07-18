package discord

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
)

var (
	State *state.State
	Self  *discord.User
	App   *discord.Application
)

func NewClient(token string) {
	State = state.NewWithIntents("Bot "+token, gateway.IntentGuilds, gateway.IntentGuildVoiceStates)
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