package internal

import (
	"dearrow-thumbnails/types"
	"errors"
	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"
	"github.com/schollz/jsonstore"
)

type Bot struct {
	Keystore *jsonstore.JSONStore
}

func (b *Bot) GetGuildData(guildID snowflake.ID) (guildData types.GuildData) {
	if err := b.Keystore.Get(guildID.String(), &guildData); err != nil {
		var noSuchKeyError jsonstore.NoSuchKeyError
		if !errors.As(err, &noSuchKeyError) {
			log.Errorf("there was an error while getting data for guild %d: ", guildID, err)
		}
	}
	return
}
