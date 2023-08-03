package apiv1

import (
	"context"
	"fmt"
	"sync"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"

	pds "github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/discord"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/player"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

type discordHandler struct {
	client *pds.Client
	player Player
	mx     *sync.Mutex
	radio  bool
	loop   bool
}

func NewDiscord(client *pds.Client, player Player) *discordHandler {
	return &discordHandler{
		client: client,
		player: player,
		mx:     &sync.Mutex{},
	}
}

func (h *discordHandler) RegisterRoutes() {
	descriptions := make(discord.StringLocales, 1)
	descriptions[discord.Russian] = "Найти видео на youtube и проиграть его"
	h.client.RegisterBoth(api.CreateCommandData{
		Name:                     "play",
		Description:              "Find the youtube video and play it",
		DescriptionLocalizations: descriptions,
		Options: discord.CommandOptions{
			&discord.StringOption{
				OptionName:  "query",
				Description: "name or link",
				Required:    true,
			},
		},
	}, h.cmdPlay, h.msgPlay)

	h.client.RegisterBoth(api.CreateCommandData{
		Name:        "skip",
		Description: "Skip current song",
	}, h.cmdSkip, h.msgSkip)

	h.client.RegisterBoth(api.CreateCommandData{
		Name:        "radio",
		Description: "Enable/Disable radio",
	}, h.cmdRadio, h.msgRadio)

	h.client.RegisterBoth(api.CreateCommandData{
		Name:        "loop",
		Description: "Enable/Disable loop",
	}, h.cmdLoop, h.msgLoop)

	h.client.RegisterBoth(api.CreateCommandData{
		Name:        "disconnect",
		Description: "Disconnect bot from voice channel",
	}, h.cmdDisconnect, h.msgDisconnect)
}

func (h *discordHandler) msgPlay(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	h.player.Input() <- player.Enqueue(c.Content, psong.ServiceYoutube, c.Author.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) cmdPlay(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	var options struct {
		Query string `discord:"query"`
	}

	if err := data.Options.Unmarshal(&options); err != nil {
		return nil, fmt.Errorf("unmarshal options: %+w", err)
	}

	h.player.Input() <- player.Enqueue(options.Query, psong.ServiceYoutube, data.Event.User.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) msgSkip(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	h.player.Input() <- player.Skip(c.Author.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) cmdSkip(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	h.player.Input() <- player.Skip(data.Event.User.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) msgRadio(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	var cmd *player.Command
	h.mx.Lock()
	h.radio = !h.radio
	if h.radio {
		cmd = player.Radio(c.Author.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	} else {
		cmd = player.RadioOff(c.Author.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	}
	h.mx.Unlock()

	h.player.Input() <- cmd
	return nil, nil
}

func (h *discordHandler) cmdRadio(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	var cmd *player.Command
	h.mx.Lock()
	h.radio = !h.radio
	if h.radio {
		cmd = player.Radio(data.Event.User.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	} else {
		cmd = player.RadioOff(data.Event.User.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	}
	h.mx.Unlock()

	h.player.Input() <- cmd
	return nil, nil
}

func (h *discordHandler) msgLoop(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	var cmd *player.Command
	h.mx.Lock()
	h.loop = !h.loop
	if h.loop {
		cmd = player.Loop(c.Author.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	} else {
		cmd = player.LoopOff(c.Author.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	}
	h.mx.Unlock()

	h.player.Input() <- cmd
	return nil, nil
}

func (h *discordHandler) cmdLoop(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	var cmd *player.Command
	h.mx.Lock()
	h.loop = !h.loop
	if h.loop {
		cmd = player.Loop(data.Event.User.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	} else {
		cmd = player.LoopOff(data.Event.User.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	}
	h.mx.Unlock()

	h.player.Input() <- cmd
	return nil, nil
}

func (h *discordHandler) msgDisconnect(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	h.player.Input() <- player.Disconnect(c.Author.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) cmdDisconnect(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, fmt.Errorf("get user voice state: %+w", err)
	}

	h.player.Input() <- player.Disconnect(data.Event.User.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}
