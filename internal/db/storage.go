package db

import (
	"MeowSentinelBot/internal/config"
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Storage interface {
	initTables(context.Context) error
	// addUser
	// createStack
	// addRecord
	// updateRecord
	// getStack
}

type SqliteStorage struct {
	db *sql.DB
}

func NewSqliteStorage(cfg *config.DB) (Storage, error) {

	dsn := fmt.Sprintf(cfg.DSN, cfg.Path)
	db, err := sql.Open(cfg.Controller, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db error: %w", err)
	}

	db.SetConnMaxLifetime(time.Hour)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db error: %w", err)
	}

	s := &SqliteStorage{db: db}

	if err := s.initTables(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate tables error: %w", err)
	}

	return s, nil
}

func (s *SqliteStorage) initTables(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}
