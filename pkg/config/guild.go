package config

type Guild struct {
	ThumbnailMode     ThumbnailMode     `db:"thumbnail_mode"`
	OriginalTitleMode OriginalTitleMode `db:"title_mode"`
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

type OriginalTitleMode int

const (
	OriginalTitleModeShown OriginalTitleMode = iota
	OriginalTitleModeHidden
)

func (t OriginalTitleMode) String() string {
	switch t {
	case OriginalTitleModeShown:
		return "Show original titles"
	case OriginalTitleModeHidden:
		return "Hide original titles"
	}
	return "Unknown"
}
