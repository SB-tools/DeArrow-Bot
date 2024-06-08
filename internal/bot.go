package internal

import (
	"dearrow-bot/dearrow"
	"errors"
	"log/slog"

	"github.com/disgoorg/snowflake/v2"
	"github.com/lmittmann/tint"
	"github.com/schollz/jsonstore"
)

type Bot struct {
	Keystore *jsonstore.JSONStore
	DeArrow  *dearrow.DeArrow
}

func (b *Bot) GetGuildData(guildID snowflake.ID) (guildData dearrow.GuildData) {
	if err := b.Keystore.Get(guildID.String(), &guildData); err != nil {
		var noSuchKeyError jsonstore.NoSuchKeyError
		if !errors.As(err, &noSuchKeyError) {
			slog.Error("error while getting data for a guild", slog.Any("guild.id", guildID), tint.Err(err))
		}
	}
	return
}
