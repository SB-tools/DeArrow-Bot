package handlers

import (
	"bytes"
	"dearrow-thumbnails/types"
	"dearrow-thumbnails/util"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/json"
	"github.com/disgoorg/log"
	"github.com/schollz/jsonstore"
	"io"
	"net/url"
	"strings"
)

func (h *Handlers) HandleBranding(event *handler.CommandEvent) error {
	data := event.SlashCommandInteractionData()
	input := data.String("video")
	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
	var videoID string
	if len(input) == 11 {
		videoID = input
	} else {
		u, err := url.Parse(input)
		if err != nil {
			return event.CreateMessage(messageBuilder.
				SetContent("Cannot parse input as URL.").
				Build())
		}
		videoID = u.Query().Get("v")
		if videoID == "" {
			path := strings.TrimSuffix(u.Path, "/")
			videoID = path[strings.LastIndex(path, "/")+1:]
		}
	}
	if videoID == "" || len(videoID) != 11 {
		return event.CreateMessage(messageBuilder.
			SetContent("Cannot extract video ID from input.").
			Build())
	}
	rs, err := util.FetchVideoBranding(event.Client().Rest().HTTPClient(), videoID, true)
	if err != nil {
		log.Errorf("there was an error while running a user branding request (%s): ", videoID, err)
		return event.CreateMessage(messageBuilder.
			SetContent("There was an error while fetching the branding.").
			Build())
	}
	defer rs.Body.Close()
	b, err := io.ReadAll(rs.Body)
	if err != nil {
		log.Errorf("there was an error while reading response body (%s): ", videoID, err)
		return event.CreateMessage(messageBuilder.
			SetContent("There was an error while decoding the response.").
			Build())
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		log.Errorf("there was an error while indenting the response (%s): ", videoID, err)
		return event.CreateMessage(messageBuilder.
			SetContent("There was an error while indenting the response.").
			Build())
	}
	indented := out.String()
	if len(indented) > 4096 {
		return event.CreateMessage(messageBuilder.
			SetContentf("Response is longer than 4096 chars (%d).", len(indented)).
			Build())
	}
	embedBuilder := discord.NewEmbedBuilder()
	embedBuilder.SetColor(0x001BFF)
	embedBuilder.SetDescriptionf("```json\n%s\n```", indented)
	return event.CreateMessage(messageBuilder.
		SetEmbeds(embedBuilder.Build()).
		Build())
}

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
