package handlers

import (
	"dearrow-bot/pkg/config"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/lmittmann/tint"
)

func (h *Handler) modeCurrentHandler(event *handler.CommandEvent, modeFunc func(guild config.Guild) string) error {
	cfg, err := h.Bot.DB.GetGuildConfig(*event.GuildID())
	messageCreate := discord.NewMessageCreate().WithEphemeral(true)
	if err != nil {
		slog.Error("dearrow: error while getting guild config", slog.Any("guild.id", *event.GuildID()), tint.Err(err))
		return event.CreateMessage(messageCreate.WithContent("There was an error while getting the guild configuration."))
	}
	return event.CreateMessage(messageCreate.WithContentf("Current mode is set to **%s**.", modeFunc(cfg)))
}
