package internal

import "github.com/disgoorg/snowflake/v2"

type Config struct {
	StoragePath   string
	DeArrowUserID snowflake.ID
}
