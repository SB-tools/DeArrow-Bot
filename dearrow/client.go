package dearrow

import (
	"dearrow-bot/config"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/json"
	"github.com/lmittmann/tint"
)

var (
	arrowRegex  = regexp.MustCompile(`(^|\s)>(\S)`)
	generateErr = errors.New("couldn't generate thumbnail")
	decodingErr = errors.New("couldn't decode branding response")
)

const (
	dearrowApiURL   = "https://sponsor.ajay.app/api/branding?videoID=%s&returnUserID=%t"
	thumbnailApiURL = "https://dearrow-thumb.ajay.app/api/v1/getThumbnail?videoID=%s&time=%.5f&generateNow=true"
)

type Client struct {
	brandingClient  *http.Client
	thumbnailClient *http.Client
}

func New(brandingClient *http.Client, thumbnailClient *http.Client) *Client {
	return &Client{
		brandingClient:  brandingClient,
		thumbnailClient: thumbnailClient,
	}
}

func (c *Client) FetchBranding(videoID string) (*BrandingResponse, error) {
	rs, err := c.FetchBrandingRaw(videoID, false)
	if err != nil {
		slog.Error("dearrow: error while running a branding request", slog.String("video.id", videoID), tint.Err(err))
		return nil, err
	}
	status := rs.StatusCode
	if status != http.StatusOK && status != http.StatusNotFound {
		slog.Warn("dearrow: received an unexpected code from a branding response", slog.Int("status.code", status), slog.String("video.id", videoID))
		return nil, err
	}
	defer rs.Body.Close()
	body, err := io.ReadAll(rs.Body)
	if err != nil {
		slog.Error("dearrow: error while reading a branding response", slog.Int("status.code", status), slog.String("video.id", videoID), tint.Err(err))
		return nil, err
	}
	var brandingResponse *BrandingResponse
	if err := json.Unmarshal(body, &brandingResponse); err != nil {
		slog.Error("dearrow: error while unmarshalling a branding response", slog.Int("status.code", status), slog.String("video.id", videoID), tint.Err(err))
		return nil, err
	}
	if brandingResponse == nil { // TODO temporary check for this behavior
		slog.Error("dearrow: could not decode a branding response", slog.String("response.body", string(body)), slog.String("video.id", videoID))
		return nil, decodingErr
	}
	return brandingResponse, nil
}

func (c *Client) FetchBrandingRaw(videoID string, returnUserID bool) (*http.Response, error) {
	return c.brandingClient.Get(fmt.Sprintf(dearrowApiURL, videoID, returnUserID))
}

func (c *Client) FetchThumbnail(videoID string, timestamp float64) (io.ReadCloser, error) {
	thumbnailURL := fmt.Sprintf(thumbnailApiURL, videoID, timestamp)

	rs, err := c.thumbnailClient.Get(thumbnailURL)
	if err != nil {
		slog.Error("dearrow: error while downloading a thumbnail", slog.String("thumbnail.url", thumbnailURL), tint.Err(err))
		return nil, err
	}
	if rs.StatusCode != http.StatusOK {
		slog.Warn("dearrow: received an unexpected code from a thumbnail response",
			slog.Int("status.code", rs.StatusCode),
			slog.String("failure.reason", rs.Header.Get("X-Failure-Reason")),
			slog.String("video.id", videoID),
			slog.String("thumbnail.url", thumbnailURL))
		return nil, generateErr
	}
	return rs.Body, nil
}

type BrandingResponse struct {
	Titles []struct {
		Title    string `json:"title"`
		Original bool   `json:"original"`
		Votes    int    `json:"votes"`
		Locked   bool   `json:"locked"`
	} `json:"titles"`
	Thumbnails []struct {
		Timestamp *float64 `json:"timestamp"`
		Original  bool     `json:"original"`
		Locked    bool     `json:"locked"`
	} `json:"thumbnails"`
	RandomTime    float64  `json:"randomTime"`
	VideoDuration *float64 `json:"videoDuration"`
}

func (b *BrandingResponse) ToReplacementData(videoID string, cfg config.Guild, embed discord.Embed, debugLogger *slog.Logger) *ReplacementData {
	embedBuilder := discord.NewEmbedBuilder()
	embedBuilder.SetAuthor(embed.Author.Name, embed.Author.URL, "")
	embedBuilder.SetTitle(embed.Title)
	embedBuilder.SetURL(embed.URL)
	embedBuilder.SetFooterText(`Tip: Use Apps -> "Delete embeds" to delete the DeArrow message.`)
	embedBuilder.SetColor(embed.Color)
	embedBuilder.SetImage(embed.Thumbnail.URL)

	original := embed.Title
	title := b.replacementTitle(original)
	timestamp := b.replacementTimestamp(cfg.ThumbnailMode, embedBuilder)
	if title == "" && timestamp == -1 { // nothing to replace
		debugLogger.Debug("dearrow: nothing to replace for video", slog.String("video.id", videoID))
		return nil
	}
	if title != "" {
		if cfg.OriginalTitleMode == config.OriginalTitleModeShown {
			embedBuilder.SetDescription("-# Original title: " + original)
		}
		embedBuilder.SetTitle(arrowRegex.ReplaceAllString(title, "$1$2"))
	}
	if timestamp != -1 {
		embedBuilder.SetImage("attachment://thumbnail-" + videoID + ".webp")
	}
	return &ReplacementData{
		Timestamp: timestamp,
		Embed:     embedBuilder.Build(),
	}
}

func (b *BrandingResponse) replacementTitle(original string) string {
	if len(b.Titles) != 0 && b.Titles[0].Votes > -1 {
		title := b.Titles[0]
		if (title.Original && !title.Locked) || title.Title == original {
			return ""
		}
		return title.Title
	}
	return ""
}

func (b *BrandingResponse) replacementTimestamp(mode config.ThumbnailMode, embedBuilder *discord.EmbedBuilder) float64 {
	if len(b.Thumbnails) != 0 {
		thumbnail := b.Thumbnails[0]
		if (thumbnail.Original && !thumbnail.Locked) || thumbnail.Timestamp == nil {
			return -1
		}
		return *thumbnail.Timestamp
	}
	switch mode {
	case config.ThumbnailModeRandomTime:
		duration := b.VideoDuration
		if duration != nil && *duration != 0 {
			return b.RandomTime * (*duration)
		}
	case config.ThumbnailModeBlank:
		embedBuilder.SetImage("")
	default: // we can ignore ThumbnailModeOriginal
	}
	return -1
}

type ReplacementData struct {
	Timestamp float64

	Embed discord.Embed
}

func (d *ReplacementData) ToEmbed() discord.Embed {
	return d.Embed
}
