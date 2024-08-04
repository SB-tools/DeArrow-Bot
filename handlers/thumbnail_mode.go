package handlers

import (
	"dearrow-bot/config"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/lmittmann/tint"
)

func (h *Handler) HandleThumbnailModeCurrent(event *handler.CommandEvent) error {
	return h.modeCurrentHandler(event, func(cfg config.Guild) string {
		return cfg.ThumbnailMode.String()
	})
}

func (h *Handler) HandleThumbnailModeSet(data discord.SlashCommandInteractionData, event *handler.CommandEvent) error {
	thumbnailMode := config.ThumbnailMode(data.Int("mode"))
	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
	if err := h.Bot.DB.UpdateGuildThumbnailMode(*event.GuildID(), thumbnailMode); err != nil {
		slog.Error("dearrow: error while updating thumbnail mode", slog.Any("mode", thumbnailMode), slog.Any("guild.id", *event.GuildID()), tint.Err(err))
		return event.CreateMessage(messageBuilder.
			SetContent("There was an error while updating the thumbnail mode.").
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("Mode has been set to **%s**.", thumbnailMode).
		Build())
}
