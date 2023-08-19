package apiv1

import (
	"context"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/pkg/errors"

	pds "github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/discord"
	"github.com/HalvaPovidlo/halva-services/internal/halva-discord-music/music/search"
	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

type discordHandler struct {
	client   *pds.Client
	player   playerService
	searcher searcher
}

func NewDiscord(client *pds.Client, player playerService, searcher searcher) *discordHandler {
	return &discordHandler{
		client:   client,
		player:   player,
		searcher: searcher,
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
		Name:        "shuffle",
		Description: "Enable/Disable shuffle",
	}, h.cmdShuffle, h.msgShuffle)

	h.client.RegisterBoth(api.CreateCommandData{
		Name:        "disconnect",
		Description: "Disconnect bot from voice channel",
	}, h.cmdDisconnect, h.msgDisconnect)
}

func (h *discordHandler) msgPlay(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get user voice state")
	}

	song, err := h.searcher.Search(ctx, &search.Request{
		Text:    c.Content,
		UserID:  c.Author.ID,
		Service: psong.ServiceYoutube,
	})
	if err != nil {
		return nil, errors.Wrap(err, "search song")
	}

	h.player.Play(song, voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) cmdPlay(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get user voice state")
	}

	var options struct {
		Query string `discord:"query"`
	}

	if err := data.Options.Unmarshal(&options); err != nil {
		return nil, errors.Wrap(err, "unmarshal options")
	}

	song, err := h.searcher.Search(ctx, &search.Request{
		Text:    options.Query,
		UserID:  data.Event.User.ID,
		Service: psong.ServiceYoutube,
	})
	if err != nil {
		return nil, errors.Wrap(err, "search song")
	}

	h.player.Play(song, voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) msgSkip(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get user voice state")
	}

	h.player.Skip(voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) cmdSkip(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get user voice state")
	}

	h.player.Skip(voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) msgRadio(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get user voice state")
	}

	h.player.RadioToggle(voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) cmdRadio(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get user voice state")
	}

	h.player.RadioToggle(voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) msgLoop(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	h.player.LoopToggle()
	return nil, nil
}

func (h *discordHandler) cmdLoop(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	h.player.LoopToggle()
	return nil, nil
}

func (h *discordHandler) msgShuffle(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	h.player.ShuffleToggle()
	return nil, nil
}

func (h *discordHandler) cmdShuffle(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	h.player.ShuffleToggle()
	return nil, nil
}

func (h *discordHandler) msgDisconnect(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, c.Author.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get user voice state")
	}

	h.player.Disconnect(voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}

func (h *discordHandler) cmdDisconnect(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	voiceState, err := h.client.VoiceState(pds.HalvaGuildID, data.Event.User.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get user voice state")
	}

	h.player.Disconnect(voiceState.ChannelID, contexts.GetTraceID(ctx))
	return nil, nil
}
