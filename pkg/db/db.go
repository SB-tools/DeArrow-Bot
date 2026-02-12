package db

import (
	"context"
	"dearrow-bot/pkg/config"
	"errors"

	"github.com/disgoorg/snowflake/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	selectQuery              = "SELECT thumbnail_mode, title_mode FROM config WHERE guild_id = $1;"
	upsertThumbnailModeQuery = "INSERT INTO config (guild_id, thumbnail_mode) VALUES ($1, $2) ON CONFLICT(guild_id) DO UPDATE SET thumbnail_mode=excluded.thumbnail_mode;"
	upsertTitleModeQuery     = "INSERT INTO config (guild_id, title_mode) VALUES ($1, $2) ON CONFLICT(guild_id) DO UPDATE SET title_mode=excluded.title_mode;"
)

type DB struct {
	pool *pgxpool.Pool
}

func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool}
}

func (db *DB) GetGuildConfig(guildID snowflake.ID) (cfg config.Guild, err error) {
	rows, _ := db.pool.Query(context.Background(), selectQuery, guildID)
	cfg, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[config.Guild])
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return
}

func (db *DB) UpdateGuildThumbnailMode(guildID snowflake.ID, mode config.ThumbnailMode) error {
	_, err := db.pool.Exec(context.Background(), upsertThumbnailModeQuery, guildID, mode)
	return err
}

func (db *DB) UpdateGuildTitleMode(guildID snowflake.ID, mode config.OriginalTitleMode) error {
	_, err := db.pool.Exec(context.Background(), upsertTitleModeQuery, guildID, mode)
	return err
}
