package handlers

import (
	"bytes"
	"dearrow-thumbnails/types"
	"dearrow-thumbnails/util"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/json"
	"github.com/schollz/jsonstore"
	"io"
	"net/url"
	"strings"
)

const (
	videoIDLen = 11
)

func (h *Handlers) HandleBranding(event *handler.CommandEvent) (err error) {
	data := event.SlashCommandInteractionData()
	input := data.String("video")
	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
	var videoID string
	if len(input) == videoIDLen {
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
	if videoID == "" || len(videoID) != videoIDLen {
		return event.CreateMessage(messageBuilder.
			SetContent("Cannot extract video ID from input.").
			Build())
	}
	rs, err := util.FetchVideoBranding(event.Client().Rest().HTTPClient(), videoID, true)
	if err != nil {
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

func (h *Handlers) HandleModeSet(event *handler.CommandEvent) (err error) {
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
