package apiv1

import (
	"context"
	"fmt"

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
}

func NewDiscord(client *pds.Client, player Player) *discordHandler {
	return &discordHandler{
		client: client,
		player: player,
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

//
//func (h *discordHandler) msgRadio(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
//	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
//	if err != nil {
//		return nil, fmt.Errorf("get user voice state: %+w", err)
//	}
//
//	h.player.Input() <- player.Radio(c.Author.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
//	return nil, nil
//}
//
//func (h *discordHandler) cmdRadio(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
//	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
//	if err != nil {
//		return nil, fmt.Errorf("get user voice state: %+w", err)
//	}
//
//	h.player.Input() <- player.Radio(data.Event.User.ID, voiceState.ChannelID, contexts.GetTraceID(ctx))
//	return nil, nil
//}
