package types

import "github.com/disgoorg/disgo/discord"

type GuildData struct {
	ThumbnailMode ThumbnailMode `json:"thumbnail_mode"`
}

type DeArrowEmbedData struct {
	Embed                   discord.Embed
	ReplacementThumbnailURL string
}

type ThumbnailMode int

const (
	ThumbnailModeRandomTime ThumbnailMode = iota
	ThumbnailModeBlank
	ThumbnailModeOriginal
)

func (t ThumbnailMode) String() string {
	switch t {
	case ThumbnailModeRandomTime:
		return "Show a screenshot from a random time"
	case ThumbnailModeBlank:
		return "Show no thumbnail"
	case ThumbnailModeOriginal:
		return "Show the original thumbnail"
	}
	return "Unknown"
}
