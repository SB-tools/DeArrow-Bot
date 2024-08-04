package handlers

import (
	"dearrow-bot/internal"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/lmittmann/tint"
)

func NewHandler(b *internal.Bot, c *internal.Config) *Handler {
	mux := handler.New()
	mux.Error(func(e *handler.InteractionEvent, err error) {
		i := e.Interaction.(discord.ApplicationCommandInteraction)
		slog.Error("dearrow: error while handling a command", slog.String("command.name", i.Data.CommandName()), tint.Err(err))
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
		r.Route("/configure", func(r handler.Router) {
			r.Group(func(r handler.Router) {
				r.Route("/thumbnails", func(r handler.Router) {
					r.Command("/current", handlers.HandleThumbnailModeCurrent)
					r.SlashCommand("/set", handlers.HandleThumbnailModeSet)
				})
			})
			r.Group(func(r handler.Router) {
				r.Route("/titles", func(r handler.Router) {
					r.Command("/current", handlers.HandleOriginalTitleModeCurrent)
					r.SlashCommand("/set", handlers.HandleOriginalTitleModeSet)
				})
			})
		})
	})
	handlers.Group(func(r handler.Router) {
		r.SlashCommand("/branding", handlers.HandleBrandingSlash)
		r.MessageCommand("/Fetch branding", handlers.HandleBrandingContext)
	})
	handlers.MessageCommand("/Delete embeds", handlers.HandleDeleteEmbeds)
	return handlers
}

type Handler struct {
	Bot    *internal.Bot
	Config *internal.Config
	handler.Router
}
