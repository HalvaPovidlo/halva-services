package discord

import (
	"context"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/HalvaPovidlo/halva-services/pkg/contexts"
)

const (
	HalvaGuildID = discord.GuildID(623964456929591297)

	MonkaS               = "<:monkaS:817041877718138891>"
	messageInternalError = ":x: **Internal error** " + MonkaS
)

type (
	MessageHandlerFunc func(ctx context.Context, c *gateway.MessageCreateEvent) (*api.SendMessageData, error)
	CommandHandlerFunc func(ctx context.Context, data cmdroute.CommandData) (*api.InteractionResponseData, error)
)

type Config struct {
	Token        string
	Prefix       string
	DebugChannel int64 `yaml:"debug_channel"`
}

var State *Client

type Client struct {
	*state.State
	router *cmdroute.Router
	self   *discord.User

	messageCommandPrefix string
	commands             []api.CreateCommandData

	debug        bool
	debugChannel discord.ChannelID

	logger *zap.Logger
}

func NewClient(cfg Config, logger *zap.Logger, debug bool) *Client {
	State = &Client{
		State: state.NewWithIntents("Bot "+cfg.Token,
			gateway.IntentGuilds,
			gateway.IntentGuildMessages,
			gateway.IntentGuildVoiceStates,
			gateway.IntentDirectMessages),
		router:               cmdroute.NewRouter(),
		messageCommandPrefix: cfg.Prefix,
		debug:                debug,
		debugChannel:         discord.ChannelID(cfg.DebugChannel),
		logger:               logger,
	}
	return State
}

func (c *Client) Connect(ctx context.Context) error {
	c.router.Use(cmdroute.Deferrable(c, cmdroute.DeferOpts{}))

	err := c.Open(ctx)
	if err != nil {
		return errors.Wrap(err, "open")
	}

	c.self, err = c.Me()
	if err != nil {
		return errors.Wrap(err, "get Me")
	}

	if err := cmdroute.OverwriteCommands(c, c.commands); err != nil {
		var httpErr *httputil.HTTPError
		if errors.As(err, &httpErr) {
			return errors.Wrapf(err, "Message: %s Body: %s", httpErr.Message, string(httpErr.Body))
		}
	}

	if err != nil {
		return errors.Errorf("connect: %w", err)
	}
	return nil
}

func (c *Client) Close() {
	_ = c.State.Close()
}

func (c *Client) RegisterBoth(cmd api.CreateCommandData, cmdHandle CommandHandlerFunc, msgHandle MessageHandlerFunc) {
	c.RegisterCommand(cmd, cmdHandle)
	c.RegisterMessageCommand(cmd.Name, msgHandle)
}

func (c *Client) RegisterCommand(cmd api.CreateCommandData, cmdHandle CommandHandlerFunc) {
	c.commands = append(c.commands, cmd)
	c.router.AddFunc(cmd.Name, func(ctx context.Context, data cmdroute.CommandData) *api.InteractionResponseData {
		defer func() {
			if e := recover(); e != nil {
				c.logger.Error("panic during command handling", zap.Any("error", e), zap.Stack("stack"))
			}
		}()

		ctx = contexts.WithCommandValues(ctx, cmd.Name, c.logger, "")
		log := contexts.GetLogger(ctx)

		start := time.Now()
		log.Info("command handled")
		response, err := cmdHandle(ctx, data)
		if response == nil && err != nil {
			return &api.InteractionResponseData{
				Content:         option.NewNullableString(messageInternalError),
				Flags:           discord.EphemeralMessage,
				AllowedMentions: &api.AllowedMentions{},
			}
		}

		if err != nil {
			log.Error("command execution failed", zap.Error(err), zap.Duration("latency", time.Since(start)))
		} else {
			log.Info("command executed", zap.Duration("elapsed", time.Since(start)))
		}

		return response
	})
}

func (c *Client) RegisterMessageCommand(name string, handle MessageHandlerFunc) {
	c.AddHandler(func(event *gateway.MessageCreateEvent) {
		defer func() {
			if e := recover(); e != nil {
				c.logger.Error("panic during message handling", zap.Any("error", e), zap.Stack("stack"))
				_, _ = c.SendMessage(event.ChannelID, messageInternalError)
			}
		}()

		if c.skip(event) {
			return
		}

		command := c.messageCommandPrefix + name
		if len(event.Content) >= len(command) && command == strings.ToLower(event.Content[0:len(command)]) {
			ctx := contexts.WithCommandValues(context.Background(), name, c.logger, "")
			log := contexts.GetLogger(ctx)

			start := time.Now()
			log.Info("command handled", zap.String("query", event.Content))
			event.Content = event.Content[len(command):]
			response, err := handle(ctx, event)
			switch {
			case response != nil:
				if _, err := c.SendMessageComplex(event.ChannelID, *response); err != nil {
					log.Error("send message failed", zap.Error(err))
				}
			case err != nil:
				if _, err := c.SendMessage(event.ChannelID, messageInternalError); err != nil {
					log.Error("send message failed", zap.Error(err))
				}
			}

			if err != nil {
				log.Error("command execution failed", zap.Error(err), zap.Duration("latency", time.Since(start)))
			} else {
				log.Info("command executed", zap.Duration("elapsed", time.Since(start)))
			}
		}
	})
}

func (c *Client) skip(event *gateway.MessageCreateEvent) bool {
	switch {
	case event.Member.User.ID == c.self.ID:
		return true
	case (event.ChannelID == c.debugChannel) != c.debug:
		return true
	}
	return false
}
