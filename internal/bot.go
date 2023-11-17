package internal

import (
	"dearrow-thumbnails/types"
	"errors"
	"github.com/disgoorg/snowflake/v2"
	"github.com/lmittmann/tint"
	"github.com/schollz/jsonstore"
	"log/slog"
	"net/http"
)

type Bot struct {
	Keystore *jsonstore.JSONStore
	Client   *http.Client
}

func (b *Bot) GetGuildData(guildID snowflake.ID) (guildData types.GuildData) {
	if err := b.Keystore.Get(guildID.String(), &guildData); err != nil {
		var noSuchKeyError jsonstore.NoSuchKeyError
		if !errors.As(err, &noSuchKeyError) {
			slog.Error("there was an error while getting data for a guild", slog.Any("guild.id", guildID), tint.Err(err))
		}
	}
	return
}
