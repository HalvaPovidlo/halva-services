package apiv1

import (
	"context"
	"fmt"
	"strconv"

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

const (
	messageSearching       = ":trumpet: **Searching** :mag_right:"
	messageSkip            = ":fast_forward: **Skipped** :thumbsup:"
	messageFound           = "**Song found** :notes:"
	messageNotFound        = ":x: **Song not found**"
	messageAgeRestriction  = ":underage: **Song is blocked**"
	messageLoopEnabled     = ":white_check_mark: **Loop enabled**"
	messageLoopDisabled    = ":x: **Loop disabled**"
	messageRadioEnabled    = ":white_check_mark: **Radio enabled**"
	messageRadioDisabled   = ":x: **Radio disabled**"
	messageShuffleEnabled  = ":white_check_mark: **Shuffle enabled**"
	messageShuffleDisabled = ":x: **Shuffle disabled**"
	messageNotVoiceChannel = ":x: **You have to be in a voice channel to use this command**"
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

	_, err = h.client.SendMessage(c.ChannelID, messageSearching)
	if err != nil {
		return nil, errors.Wrap(err, "send message")
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
	return &api.SendMessageData{Content: fmt.Sprintf("%s `%s - %s` %s", messageFound, song.Artist, song.Title, intToEmoji(song.Count))}, nil
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
	return &api.SendMessageData{Content: messageSkip}, nil
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

	if h.player.RadioToggle(voiceState.ChannelID, contexts.GetTraceID(ctx)) {
		return &api.SendMessageData{Content: messageRadioEnabled}, nil
	}
	return &api.SendMessageData{Content: messageRadioDisabled}, nil
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
	if h.player.LoopToggle() {
		return &api.SendMessageData{Content: messageLoopEnabled}, nil
	}
	return &api.SendMessageData{Content: messageLoopDisabled}, nil
}

func (h *discordHandler) cmdLoop(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error) {
	h.player.LoopToggle()
	return nil, nil
}

func (h *discordHandler) msgShuffle(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error) {
	if h.player.ShuffleToggle() {
		return &api.SendMessageData{Content: messageShuffleEnabled}, nil
	}
	return &api.SendMessageData{Content: messageShuffleDisabled}, nil
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

func intToEmoji(n int64) string {
	if n == 0 {
		return ""
	}
	number := strconv.Itoa(int(n))
	res := ""
	for i := range number {
		res += digitAsEmoji(string(number[i]))
	}
	return res
}

func digitAsEmoji(digit string) string {
	switch digit {
	case "1":
		return "1️⃣"
	case "2":
		return "2️⃣"
	case "3":
		return "3️⃣"
	case "4":
		return "4️⃣"
	case "5":
		return "5️⃣"
	case "6":
		return "6️⃣"
	case "7":
		return "7️⃣"
	case "8":
		return "8️⃣"
	case "9":
		return "9️⃣"
	case "0":
		return "0️⃣"
	}
	return ""
}
