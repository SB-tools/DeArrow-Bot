package handlers

import (
	"bytes"
	"dearrow-thumbnails/types"
	"dearrow-thumbnails/util"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/json"
	"github.com/schollz/jsonstore"
)

var (
	videoIDRegex = regexp.MustCompile(`[a-zA-Z0-9-_]{11}`)
)

func (h *Handler) HandleBranding(event *handler.CommandEvent) (err error) {
	data := event.SlashCommandInteractionData()
	videoID := videoIDRegex.FindString(data.String("video"))
	messageBuilder := discord.NewMessageCreateBuilder()
	if videoID == "" {
		return event.CreateMessage(messageBuilder.
			SetContent("Cannot extract video ID from input.").
			SetEphemeral(true).
			Build())
	}
	if err = event.DeferCreateMessage(true); err != nil {
		return err
	}
	rs, err := util.FetchVideoBranding(h.Bot.Client, videoID, true)
	if err != nil {
		if os.IsTimeout(err) {
			_, err = event.CreateFollowupMessage(messageBuilder.
				SetContent("DeArrow API failed to respond within 5 seconds.").
				SetEphemeral(true).
				Build())
			return nil
		}
		return err
	}
	defer rs.Body.Close()
	b, err := io.ReadAll(rs.Body)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err = json.Indent(&out, b, "", "  "); err != nil {
		return err
	}
	content := fmt.Sprintf("```json\n%s\n```", out.String())
	if len(content) > 4096 {
		_, err = event.CreateFollowupMessage(messageBuilder.
			SetContentf("Response is longer than 4096 chars (%d).", len(content)).
			SetEphemeral(true).
			Build())
		return err
	}
	hide, ok := data.OptBool("hide")
	if !ok {
		hide = true
	}
	embedBuilder := discord.NewEmbedBuilder()
	embedBuilder.SetColor(0x001BFF)
	embedBuilder.SetDescription(content)
	_, err = event.CreateFollowupMessage(messageBuilder.
		SetEmbeds(embedBuilder.Build()).
		SetEphemeral(hide).
		Build())
	return err
}

func (h *Handler) HandleModeGet(event *handler.CommandEvent) error {
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("Current mode is set to **%s**.", h.Bot.GetGuildData(*event.GuildID()).ThumbnailMode).
		SetEphemeral(true).
		Build())
}

func (h *Handler) HandleModeSet(event *handler.CommandEvent) (err error) {
	data := event.SlashCommandInteractionData()
	guildID := event.GuildID()
	thumbnailMode := types.ThumbnailMode(data.Int("mode"))
	err = h.Bot.Keystore.Set(guildID.String(), types.GuildData{
		ThumbnailMode: thumbnailMode,
	})
	if err != nil {
		return err
	}
	if err = jsonstore.Save(h.Bot.Keystore, h.Config.StoragePath); err != nil {
		return err
	}
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("Mode has been set to **%s**.", thumbnailMode).
		SetEphemeral(true).
		Build())
}
