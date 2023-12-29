package handlers

import (
	"bytes"
	"dearrow-thumbnails/util"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/json"
)

var (
	videoIDRegex = regexp.MustCompile(`[a-zA-Z0-9-_]{11}`)
)

func (h *Handler) HandleBrandingSlash(event *handler.CommandEvent) error {
	data := event.SlashCommandInteractionData()
	videoID := videoIDRegex.FindString(data.String("video"))
	hide, ok := data.OptBool("hide")
	if !ok {
		hide = true
	}
	return h.handleBranding(event, videoID, hide)
}

func (h *Handler) HandleBrandingContext(event *handler.CommandEvent) error {
	var videoID string

	data := event.MessageCommandInteractionData()
	message := data.TargetMessage()
	embeds := message.Embeds
	if len(embeds) != 0 {
		videoID = util.ParseVideoID(embeds[0])
	}
	if videoID == "" {
		videoID = videoIDRegex.FindString(message.Content)
	}
	return h.handleBranding(event, videoID, true)
}

func (h *Handler) handleBranding(event *handler.CommandEvent, videoID string, hide bool) error {
	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
	if videoID == "" {
		return event.CreateMessage(messageBuilder.
			SetContent("Cannot extract video ID from input.").
			Build())
	}
	rs, err := util.FetchVideoBranding(h.Bot.Client, videoID, true)
	if err != nil {
		if os.IsTimeout(err) {
			return event.CreateMessage(messageBuilder.
				SetContent("DeArrow API failed to respond within 2 seconds.").
				Build())
		}
		return err
	}
	defer rs.Body.Close()
	b, err := io.ReadAll(rs.Body)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		return err
	}
	content := fmt.Sprintf("```json\n%s\n```", out.String())
	if len(content) > 4096 {
		return event.CreateMessage(messageBuilder.
			SetContentf("Response is longer than 4096 chars (%d).", len(content)).
			Build())
	}
	embedBuilder := discord.NewEmbedBuilder()
	embedBuilder.SetColor(0x001BFF)
	embedBuilder.SetDescription(content)
	return event.CreateMessage(messageBuilder.
		SetEmbeds(embedBuilder.Build()).
		SetEphemeral(hide).
		Build())
}
