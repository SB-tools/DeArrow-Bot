package handlers

import (
	"bytes"
	"dearrow-bot/pkg/util"
	"io"
	"net/http"
	"os"
	"regexp"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/json"
)

var (
	videoIDRegex = regexp.MustCompile(`[a-zA-Z0-9-_]{11}\b`)
)

const (
	lengthLimit = 4096
)

func (h *Handler) HandleBrandingSlash(data discord.SlashCommandInteractionData, event *handler.CommandEvent) error {
	videoID := videoIDRegex.FindString(data.String("video"))
	hide, ok := data.OptBool("hide")
	if !ok {
		hide = true
	}
	return h.handleBranding(event, videoID, hide)
}

func (h *Handler) HandleBrandingContext(data discord.MessageCommandInteractionData, event *handler.CommandEvent) error {
	var videoID string

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
	messageCreate := discord.NewMessageCreate().WithEphemeral(true)
	if videoID == "" {
		return event.CreateMessage(messageCreate.WithContent("Cannot extract video ID from input."))
	}
	rs, err := h.Bot.Client.FetchBrandingRaw(videoID, true)
	if err != nil {
		if os.IsTimeout(err) {
			return event.CreateMessage(messageCreate.WithContent("DeArrow API failed to respond within 2 seconds."))
		}
		return err
	}
	status := rs.StatusCode
	if status != http.StatusOK && status != http.StatusNotFound {
		return event.CreateMessage(messageCreate.WithContentf("DeArrow API returned a non-OK code: **%d**", status))
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
	content := "```json\n" + out.String() + "\n```"
	if len(content) > lengthLimit {
		return event.CreateMessage(messageCreate.WithContentf("Response is longer than **%d** chars (**%d**). See the full response [here](%s).", lengthLimit, len(content), rs.Request.URL))
	}
	embedBuilder := discord.NewEmbedBuilder()
	embedBuilder.SetColor(0x001BFF)
	embedBuilder.SetDescription(content)
	return event.CreateMessage(messageCreate.WithEmbeds(embedBuilder.Build()).WithEphemeral(hide))
}
