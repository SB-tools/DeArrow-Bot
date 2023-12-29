package util

import (
	"net/url"

	"github.com/disgoorg/disgo/discord"
)

func ParseVideoID(embed discord.Embed) string {
	u, _ := url.Parse(embed.URL)
	return u.Query().Get("v")
}
