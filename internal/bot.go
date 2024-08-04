package internal

import (
	"dearrow-bot/db"
	"dearrow-bot/dearrow"

	"github.com/disgoorg/snowflake/v2"
)

type Bot struct {
	DB       *db.DB
	Client   *dearrow.Client
	ReplyMap map[snowflake.ID]snowflake.ID
}
