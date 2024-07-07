package util

import (
	"net/url"

	"github.com/disgoorg/disgo/discord"
)

func ExtractVideoID(embed discord.Embed) string {
	provider := embed.Provider
	if provider == nil || provider.Name != "YouTube" {
		return ""
	}
	u, _ := url.Parse(embed.URL)
	return u.Query().Get("v")
}
