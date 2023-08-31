package handlers

import (
	"dearrow-thumbnails/internal"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/handler"
	"github.com/lmittmann/tint"
	"log/slog"
)

func NewHandler(b *internal.Bot, c *internal.Config) *Handler {
	mux := handler.New()
	mux.Error(func(e *events.InteractionCreate, err error) {
		i := e.Interaction.(discord.ApplicationCommandInteraction)
		slog.Error("there was an error while handling a command", slog.String("command.name", i.Data.CommandName()), tint.Err(err))
		_ = e.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
			SetContentf("There was an error while handling the command: %v", err).
			SetEphemeral(true).
			Build())
	})
	handlers := &Handler{
		Bot:    b,
		Config: c,
		Router: mux,
	}
	handlers.Group(func(r handler.Router) {
		r.Route("/dearrow", func(r handler.Router) {
			r.Route("/mode", func(r handler.Router) {
				r.Command("/get", handlers.HandleModeGet)
				r.Command("/set", handlers.HandleModeSet)
			})
			r.Command("/branding", handlers.HandleBranding)
		})
	})
	handlers.Command("/Delete embeds", handlers.HandleDeleteEmbeds)
	return handlers
}

type Handler struct {
	Bot    *internal.Bot
	Config *internal.Config
	handler.Router
}
