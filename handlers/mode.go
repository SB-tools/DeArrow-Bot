package handlers

import (
	"dearrow-bot/dearrow"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/schollz/jsonstore"
)

func (h *Handler) HandleModeGet(event *handler.CommandEvent) error {
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("Current mode is set to **%s**.", h.Bot.GetGuildData(*event.GuildID()).ThumbnailMode).
		SetEphemeral(true).
		Build())
}

func (h *Handler) HandleModeSet(event *handler.CommandEvent) error {
	data := event.SlashCommandInteractionData()
	guildID := event.GuildID()
	thumbnailMode := dearrow.ThumbnailMode(data.Int("mode"))
	if err := h.Bot.Keystore.Set(guildID.String(), dearrow.GuildData{
		ThumbnailMode: thumbnailMode,
	}); err != nil {
		return err
	}
	if err := jsonstore.Save(h.Bot.Keystore, h.Config.StoragePath); err != nil {
		return err
	}
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("Mode has been set to **%s**.", thumbnailMode).
		SetEphemeral(true).
		Build())
}
