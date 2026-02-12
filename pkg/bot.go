package pkg

import (
	"dearrow-bot/pkg/db"
	"dearrow-bot/pkg/dearrow"

	"github.com/disgoorg/snowflake/v2"
)

type Bot struct {
	DB       *db.DB
	Client   *dearrow.Client
	ReplyMap map[snowflake.ID]snowflake.ID
}
