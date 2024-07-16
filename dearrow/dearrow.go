package dearrow

import (
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
)

const (
	dearrowApiURL   = "https://sponsor.ajay.app/api/branding?videoID=%s&returnUserID=%t"
	thumbnailApiURL = "https://dearrow-thumb.ajay.app/api/v1/getThumbnail?videoID=%s&time=%.5f&generateNow=true"
)

type DeArrow struct {
	brandingClient  *http.Client
	thumbnailClient *http.Client
}

func New(brandingClient *http.Client, thumbnailClient *http.Client) *DeArrow {
	return &DeArrow{
		brandingClient:  brandingClient,
		thumbnailClient: thumbnailClient,
	}
}

func (d *DeArrow) FetchBranding(videoID string) (*BrandingResponse, error) {
	rs, err := d.FetchBrandingRaw(videoID, false)
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
	var brandingResponse *BrandingResponse
	if err := json.NewDecoder(rs.Body).Decode(&brandingResponse); err != nil {
		slog.Error("dearrow: error while decoding a branding response", slog.Int("status.code", status), slog.String("video.id", videoID), tint.Err(err))
		return nil, err
	}
	return brandingResponse, nil
}

func (d *DeArrow) FetchBrandingRaw(videoID string, returnUserID bool) (*http.Response, error) {
	return d.brandingClient.Get(fmt.Sprintf(dearrowApiURL, videoID, returnUserID))
}

func (d *DeArrow) FetchThumbnail(videoID string, timestamp float64) (io.ReadCloser, error) {
	thumbnailURL := formatThumbnailURL(videoID, timestamp)

	rs, err := d.thumbnailClient.Get(thumbnailURL)
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

func (b *BrandingResponse) ToReplacementData(videoID string, guildData GuildData, embed discord.Embed, debugLogger *slog.Logger) *ReplacementData {
	embedBuilder := &discord.EmbedBuilder{Embed: embed}
	embedBuilder.SetImage(embed.Thumbnail.URL)
	embedBuilder.SetThumbnail("")
	embedBuilder.SetDescription("")

	original := embed.Title
	title := b.replacementTitle(original)
	timestamp := b.replacementTimestamp(guildData.ThumbnailMode, embedBuilder)
	if title == "" && timestamp == -1 { // nothing to replace
		debugLogger.Debug("dearrow: nothing to replace for video", slog.String("video.id", videoID))
		return nil
	}
	if title != "" {
		embedBuilder.SetFooterText("Original title: " + original)
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

func (b *BrandingResponse) replacementTimestamp(mode ThumbnailMode, embedBuilder *discord.EmbedBuilder) float64 {
	if len(b.Thumbnails) != 0 {
		thumbnail := b.Thumbnails[0]
		if (thumbnail.Original && !thumbnail.Locked) || thumbnail.Timestamp == nil {
			return -1
		}
		return *thumbnail.Timestamp
	}
	switch mode {
	case ThumbnailModeRandomTime:
		duration := b.VideoDuration
		if duration != nil && *duration != 0 {
			return b.RandomTime * (*duration)
		}
	case ThumbnailModeBlank:
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

func formatThumbnailURL(videoID string, timestamp float64) string {
	return fmt.Sprintf(thumbnailApiURL, videoID, timestamp)
}
