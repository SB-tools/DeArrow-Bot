package handlers

import (
	"dearrow-thumbnails/internal"
	"github.com/disgoorg/disgo/handler"
)

func NewHandlers(b *internal.Bot, c *internal.Config) *Handlers {
	handlers := &Handlers{
		Bot:    b,
		Config: c,
		Router: handler.New(),
	}
	handlers.Group(func(r handler.Router) {
		r.Route("/dearrow", func(r handler.Router) {
			r.Route("/mode", func(r handler.Router) {
				r.Command("/get", handlers.HandleModeGet)
				r.Command("/set", handlers.HandleModeSet)
			})
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
