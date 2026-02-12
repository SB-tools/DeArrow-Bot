package handlers

import (
	"dearrow-bot/pkg/config"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/lmittmann/tint"
)

func (h *Handler) HandleOriginalTitleModeCurrent(event *handler.CommandEvent) error {
	return h.modeCurrentHandler(event, func(cfg config.Guild) string {
		return cfg.OriginalTitleMode.String()
	})
}

func (h *Handler) HandleOriginalTitleModeSet(data discord.SlashCommandInteractionData, event *handler.CommandEvent) error {
	originalTitleMode := config.OriginalTitleMode(data.Int("mode"))
	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
	if err := h.Bot.DB.UpdateGuildTitleMode(*event.GuildID(), originalTitleMode); err != nil {
		slog.Error("dearrow: error while updating title mode", slog.Any("mode", originalTitleMode), slog.Any("guild.id", *event.GuildID()), tint.Err(err))
		return event.CreateMessage(messageBuilder.
			SetContent("There was an error while updating the title mode.").
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("Mode has been set to **%s**.", originalTitleMode).
		Build())
}
