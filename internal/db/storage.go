package db

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/model"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

type Storage interface {
	initTables(context.Context) error
	AddUser(ctx context.Context, user model.User) error
	CreateStack(ctx context.Context, user model.User) error
	AddRecord(ctx context.Context, user model.User) error
	UpdateRecord(ctx context.Context, user model.User) error
	GetStack(ctx context.Context, user model.User) (string, error)
	GetRecord(ctx context.Context, user model.User) (string, error)
	UpdateRecent(ctx context.Context, user model.User) error
	Close()
}

var queryInitSchema = []string{ // TODO rewrite user_id to owner_id
	`CREATE TABLE IF NOT EXISTS stacks (
    					id INTEGER PRIMARY KEY AUTOINCREMENT
    					,user_id INTEGER NOT NULL
    					,title TEXT NOT NULL
    					,created_at DATETIME DEFAULT CURRENT_TIMESTAMP
                        ,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
                    	,FOREIGN KEY (user_id) REFERENCES users(telegram_id) ON DELETE CASCADE
		);`,
	`CREATE TABLE IF NOT EXISTS media_records (
		  				id INTEGER PRIMARY KEY AUTOINCREMENT
		  				,stack_id INTEGER NULL
		  				,title TEXT NOT NULL
		  				,type TEXT NOT NULL
		  				,status TEXT NOT NULL DEFAULT '{}'
		  				,created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		  				,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		  				,FOREIGN KEY (stack_id) REFERENCES stacks (id) ON DELETE CASCADE  
		);`,
	`CREATE TABLE IF NOT EXISTS users (
		                telegram_id INTEGER PRIMARY KEY 
		    			,stack_ids TEXT NOT NULL DEFAULT '{}'
		            	,recent_media TEXT NOT NULL DEFAULT '{}'
                        ,created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		  				,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
}

const (
	// ================ DDL ================
	// ================ DML ================
	queryGetStack = `
		SELECT * FROM media_records
					WHERE stack_id=?`
	queryGetOwnedStacks = `
		SELECT id, title FROM stacks
					WHERE user_id=?`
	queryGetUserStacks = `
		SELECT stack_ids FROM users
					WHERE telegram_id=?`
	queryAddUser = `
		INSERT INTO users(telegram_id, stack_ids)
					VALUES(?,?)
					ON CONFLICT(telegram_id) DO NOTHING;`
	queryCreateStack = `
		INSERT INTO stacks(user_id, title)
					VALUES(?,?)`
	queryAddRecord = `
		INSERT INTO media_records(stack_id, title, type, status)
					VALUES(?,?,?,?)`
	queryUpdateUserStacks = `
		UPDATE users
					SET stack_ids = ?
					WHERE telegram_id=?`
	queryUpdateRecord = `
		UPDATE media_records
					SET title=?, type=?, status=?
					WHERE stack_id=? AND title=?`
	queryUpdateRecent = `
		UPDATE users
					SET recent_media = ?
					WHERE telegram_id = ?`
)

type SqliteStorage struct {
	db *sql.DB
}

func NewSqliteStorage(cfg *config.DB) (Storage, error) {

	// if file exists
	fileExists, err := checkIfExists(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("file check error: %w", err)
	}

	// db descriptor configuration
	dsn := fmt.Sprintf(cfg.DSN, cfg.Path)
	db, err := sql.Open(cfg.Controller, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db error: %w", err)
	}

	db.SetConnMaxLifetime(time.Hour)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Hour) //TODO change in prod to time.Second
	defer cancel()

	// if db is accessible
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db error: %w", err)
	}

	// file integrity
	if fileExists {
		if err := verifyDBIntegrity(ctx, db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("database is corrupted: %w", err)
		}
	}

	s := &SqliteStorage{db: db}

	// init / migrations
	if err := s.initTables(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("tables migration / initialization error: %w", err)
	}

	return s, nil
}

func checkIfExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("path %q is a directory, not a file", path)
		}
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func verifyDBIntegrity(ctx context.Context, db *sql.DB) error {
	var res string
	row := db.QueryRowContext(
		ctx,
		"PRAGMA integrity_check;",
	)
	if err := row.Scan(&res); err != nil {
		return fmt.Errorf("query integrity error: %w", err)
	}
	if res != "ok" {
		return fmt.Errorf("sqlite integrity result (fail): %s", res)
	}
	return nil
}

func (s *SqliteStorage) initTables(ctx context.Context) error {

	for i, query := range queryInitSchema {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("migration #%d failed: %w", i+1, err)
		}
	}
	return nil
}

func (s *SqliteStorage) AddUser(ctx context.Context, user model.User) (err error) {
	_, err = s.db.ExecContext(ctx, queryAddUser, user.GetTelegramID())
	return
}

func (s *SqliteStorage) CreateStack(ctx context.Context, user model.User) (err error) {
	_, err = s.db.ExecContext(ctx, queryCreateStack, user.GetTelegramID(), user.GetData())
	return
}

func (s *SqliteStorage) AddRecord(ctx context.Context, user model.User) error {
	foo, err := user.GetDataForNewRecord()
	_, err = s.db.ExecContext(ctx, queryAddRecord, user.GetData(), user.GetTelegramID())
}

func (s *SqliteStorage) UpdateRecord(ctx context.Context, user model.User) error {
	//TODO implement me
	panic("implement me")
}

func (s *SqliteStorage) GetStack(ctx context.Context, user model.User) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (s *SqliteStorage) GetRecord(ctx context.Context, user model.User) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (s *SqliteStorage) UpdateRecent(ctx context.Context, user model.User) (err error) {
	_, err = s.db.ExecContext(ctx, queryUpdateRecord, user.GetData(), user.GetTelegramID())
	return
}

func (s *SqliteStorage) Close() {
	_ = s.db.Close()
}
