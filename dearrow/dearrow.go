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
		slog.Error("error while running a branding request", slog.String("video.id", videoID), tint.Err(err))
		return nil, err
	}
	status := rs.StatusCode
	if status != http.StatusOK && status != http.StatusNotFound {
		slog.Warn("received an unexpected code from a branding response", slog.Int("status.code", status), slog.String("video.id", videoID))
		return nil, err
	}
	defer rs.Body.Close()
	var brandingResponse *BrandingResponse
	if err := json.NewDecoder(rs.Body).Decode(&brandingResponse); err != nil {
		slog.Error("error while decoding a branding response", slog.Int("status.code", status), slog.String("video.id", videoID), tint.Err(err))
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
		slog.Error("error while downloading a thumbnail", slog.String("thumbnail.url", thumbnailURL), tint.Err(err))
		return nil, err
	}
	if rs.StatusCode != http.StatusOK {
		slog.Warn("received an unexpected code from a thumbnail response",
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
	} `json:"titles"`
	Thumbnails []struct {
		Timestamp float64 `json:"timestamp"`
		Original  bool    `json:"original"`
	} `json:"thumbnails"`
	RandomTime    float64  `json:"randomTime"`
	VideoDuration *float64 `json:"videoDuration"`
}

func (b *BrandingResponse) ToReplacementData(videoID string, guildData GuildData, embed discord.Embed) *ReplacementData {
	builder := NewReplacementBuilder()
	builder.WithVideoID(videoID)

	embedBuilder := &discord.EmbedBuilder{Embed: embed}
	embedBuilder.SetImage(embed.Thumbnail.URL)
	embedBuilder.SetThumbnail("")
	embedBuilder.SetDescription("")
	builder.WithEmbedBuilder(embedBuilder)

	title := b.replacementTitle()
	timestamp := b.replacementTimestamp(guildData.ThumbnailMode, embedBuilder)
	if title == "" && timestamp == -1 { // nothing to replace
		return nil
	}
	if title != "" {
		builder.SetTitle(title)
	}
	if timestamp != -1 {
		builder.SetTimestamp(timestamp)
	}
	return builder.Build()
}

func (b *BrandingResponse) replacementTitle() string {
	if len(b.Titles) != 0 && !b.Titles[0].Original && b.Titles[0].Votes > -1 {
		return b.Titles[0].Title
	}
	return ""
}

func (b *BrandingResponse) replacementTimestamp(mode ThumbnailMode, embedBuilder *discord.EmbedBuilder) float64 {
	if len(b.Thumbnails) != 0 && !b.Thumbnails[0].Original {
		return b.Thumbnails[0].Timestamp
	}
	switch mode {
	case ThumbnailModeRandomTime:
		duration := b.VideoDuration
		if duration != nil && *duration != 0 {
			return b.RandomTime * (*duration)
		}
	case ThumbnailModeBlank:
		embedBuilder.SetImage("")
		return -1
	default: // we can ignore ThumbnailModeOriginal
	}
	return -1
}

type ReplacementData struct {
	ReplacementThumbnailFunc func(dearrow *DeArrow) (io.ReadCloser, error)

	Embed discord.Embed
}

func (d *ReplacementData) ToEmbed() discord.Embed {
	return d.Embed
}

func NewReplacementBuilder() *ReplacementBuilder {
	return &ReplacementBuilder{}
}

type ReplacementBuilder struct {
	videoID string

	thumbnailFunc func(dearrow *DeArrow) (io.ReadCloser, error)

	embedBuilder *discord.EmbedBuilder
}

func (b *ReplacementBuilder) WithVideoID(videoID string) *ReplacementBuilder {
	b.videoID = videoID
	return b
}

func (b *ReplacementBuilder) WithEmbedBuilder(embedBuilder *discord.EmbedBuilder) *ReplacementBuilder {
	b.embedBuilder = embedBuilder
	return b
}

func (b *ReplacementBuilder) SetTitle(title string) *ReplacementBuilder {
	b.embedBuilder.SetFooterText("Original title: " + b.embedBuilder.Title)
	b.embedBuilder.SetTitle(arrowRegex.ReplaceAllString(title, "$1$2"))
	return b
}

func (b *ReplacementBuilder) SetTimestamp(timestamp float64) *ReplacementBuilder {
	b.thumbnailFunc = func(dearrow *DeArrow) (io.ReadCloser, error) {
		return dearrow.FetchThumbnail(b.videoID, timestamp)
	}
	b.embedBuilder.SetImagef("attachment://thumbnail-%s.webp", b.videoID)
	return b
}

func (b *ReplacementBuilder) Build() *ReplacementData {
	return &ReplacementData{
		ReplacementThumbnailFunc: b.thumbnailFunc,
		Embed:                    b.embedBuilder.Build(),
	}
}

func formatThumbnailURL(videoID string, timestamp float64) string {
	return fmt.Sprintf(thumbnailApiURL, videoID, timestamp)
}
