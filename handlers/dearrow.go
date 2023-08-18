package handlers

import (
	"dearrow-thumbnails/types"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/log"
	"github.com/schollz/jsonstore"
)

func (h *Handlers) HandleModeGet(event *handler.CommandEvent) error {
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("Current mode is set to **%s**.", h.Bot.GetGuildData(*event.GuildID()).ThumbnailMode).
		SetEphemeral(true).
		Build())
}

func (h *Handlers) HandleModeSet(event *handler.CommandEvent) error {
	data := event.SlashCommandInteractionData()
	guildID := event.GuildID()
	thumbnailMode := types.ThumbnailMode(data.Int("mode"))
	err := h.Bot.Keystore.Set(guildID.String(), types.GuildData{
		ThumbnailMode: thumbnailMode,
	})
	if err != nil {
		log.Errorf("there was an error while setting mode %d for guild %d: ", thumbnailMode, guildID, err)
		return nil
	}
	if err := jsonstore.Save(h.Bot.Keystore, h.Config.StoragePath); err != nil {
		log.Errorf("there was an error while saving data for guild %d: ", guildID, err)
		return nil
	}
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("Mode has been set to **%s**.", thumbnailMode).
		SetEphemeral(true).
		Build())
}
