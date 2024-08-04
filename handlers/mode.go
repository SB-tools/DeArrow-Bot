package handlers

import (
	"dearrow-bot/config"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/lmittmann/tint"
)

func (h *Handler) modeCurrentHandler(event *handler.CommandEvent, modeFunc func(guild config.Guild) string) error {
	cfg, err := h.Bot.DB.GetGuildConfig(*event.GuildID())

	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
	if err != nil {
		slog.Error("dearrow: error while getting guild config", slog.Any("guild.id", *event.GuildID()), tint.Err(err))
		return event.CreateMessage(messageBuilder.
			SetContent("There was an error while getting the guild configuration.").
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("Current mode is set to **%s**.", modeFunc(cfg)).
		Build())
}
