package handlers

import (
	"dearrow-thumbnails/internal"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/log"
)

func NewHandlers(b *internal.Bot, c *internal.Config) *Handlers {
	mux := handler.New()
	mux.Error(func(e *events.InteractionCreate, err error) {
		i := e.Interaction.(discord.ApplicationCommandInteraction)
		log.Errorf("there was an error while handling command /%s: %v", i.Data.CommandName(), err)
		_ = e.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
			SetContentf("There was an error while handling the command: %v", err).
			SetEphemeral(true).
			Build())
	})
	handlers := &Handlers{
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
	handlers.Group(func(r handler.Router) {
		r.Command("/Delete embeds", handlers.HandleDeleteEmbeds)
	})
	return handlers
}

type Handlers struct {
	Bot    *internal.Bot
	Config *internal.Config
	handler.Router
}
