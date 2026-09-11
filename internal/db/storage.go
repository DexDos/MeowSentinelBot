package db

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/model"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

type Storage interface {
	initTables(context.Context) error
	AddUser(ctx context.Context, user model.User) error
	GetUser(ctx context.Context, username string) (model.TelegramUser, error)
	CreateStack(ctx context.Context, user model.User) error
	AddRecord(ctx context.Context, user model.User) error
	UpdateRecordTitle(ctx context.Context, user model.User, recId int) error
	UpdateRecordType(ctx context.Context, user model.User, recId int) error
	UpdateRecordStatus(ctx context.Context, user model.User, recId int) error
	GetUserStackIDs(ctx context.Context, user model.User) ([]int, error)
	GetOwnedStacks(ctx context.Context, user model.User) ([]model.Stack, error)
	GetStack(ctx context.Context, stackID int64) (*model.Stack, error)
	GetStackRecords(ctx context.Context, stackID int64) ([]model.Entry, error)
	UpdateRecent(ctx context.Context, user model.User) error
	UpdateUserStacks(ctx context.Context, user model.User) error
	Close()
}

var queryInitSchema = []string{
	`CREATE TABLE IF NOT EXISTS stacks (
    					id INTEGER PRIMARY KEY AUTOINCREMENT
    					,owner_id INTEGER NOT NULL
    					,title TEXT NOT NULL
    					,created_at DATETIME DEFAULT CURRENT_TIMESTAMP
                        ,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
                    	,FOREIGN KEY (owner_id) REFERENCES users(telegram_id) ON DELETE CASCADE
	);`,
	`CREATE TABLE IF NOT EXISTS entries (
		  				id INTEGER PRIMARY KEY AUTOINCREMENT
		  				,stack_id INTEGER NULL
		  				,title TEXT NOT NULL
		  				,type TEXT NOT NULL
		  				,current_season INTEGER NOT NULL
                        ,current_episode INTEGER NOT NULL
		  				,created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		  				,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		  				,FOREIGN KEY (stack_id) REFERENCES stacks (id) ON DELETE CASCADE  
	);`,
	`CREATE TABLE IF NOT EXISTS entry_ratings (
    					id INTEGER PRIMARY KEY AUTOINCREMENT
                    	,user_id INTEGER NOT NULL
                        ,entry_id INTEGER NOT NULL
                        ,rating INTEGER DEFAULT 0
                        ,FOREIGN KEY (user_id) REFERENCES users (telegram_id) ON DELETE CASCADE
                        ,FOREIGN KEY (entry_id) REFERENCES entries (id) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS users (
		                telegram_id INTEGER PRIMARY KEY 
		            	,recent_media TEXT NOT NULL DEFAULT '{}'
                        ,created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		  				,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE IF NOT EXISTS user_stacks (
    					id INTEGER PRIMARY KEY AUTOINCREMENT
                    	,user_id INTEGER NOT NULL
                        ,stack_id INTEGER NOT NULL
                        ,FOREIGN KEY (user_id) REFERENCES users (telegram_id) ON DELETE CASCADE
                    	,FOREIGN KEY (stack_id) REFERENCES stacks (id) ON DELETE CASCADE
	);`,
}

const (
	// ================ DDL ================
	// ================ DML ================

	queryGetStackRecords = `
		SELECT * FROM entries --TODO UPDATE *
					WHERE stack_id=?`
	queryGetStackById = `
		SELECT 
		    id
		    ,owner_id
		    ,title
			,created_at
			,updated_at
		FROM stacks --TODO UPDATE *
					WHERE id=?`
	queryGetOwnedStacks = `
		SELECT 
		    id
		    ,owner_id
		    ,title
			,created_at
			,updated_at
		FROM stacks
		WHERE owner_id=?`
	queryGetUserStacks = `
		SELECT 
		    s.id
		    ,owner_id
		    ,title
		    ,created_at
		    ,updated_at 
		FROM stacks s
		JOIN user_stacks us ON us.stack_id=s.id
		WHERE us.user_id=?`
	queryAddUser = `
		INSERT INTO users(telegram_id)
					VALUES(?)
					ON CONFLICT(telegram_id) DO NOTHING;`
	queryCreateStack = `
		INSERT INTO stacks(owner_id, title)
		VALUES(?,?)`
	queryAddRecord = `
		INSERT INTO entries(
		                    stack_id, 
		                    title, 
		                    type, 
		                    current_season, 
		                    current_episode)
		VALUES(?,?,?,?,?)`
	queryAddToUserStacks = `
		INSERT INTO user_stacks(user_id, stack_id)
		VALUES (?,?)`
	queryDeleteFromUserStacks = `
		DELETE FROM user_stacks
		WHERE user_id=? AND stack_id=?`
	queryUpdateRecordTitle = `
		UPDATE entries
		SET title=?
		WHERE id=?`
	queryUpdateRecordType = `
		UPDATE entries
		SET type=?
		WHERE id=?`
	queryUpdateRecordStatus = `
		UPDATE entries
		SET current_season=?, current_episode=?
		WHERE id=?`
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
	_, err = s.db.ExecContext(
		ctx,
		queryAddUser,
		user.GetTelegramID(),
	)
	return
}
func (s *SqliteStorage) GetUser(ctx context.Context, username string) (user model.TelegramUser, err error) {
	panic("unimplemented")
}
func (s *SqliteStorage) CreateStack(ctx context.Context, user model.User) (err error) {
	_, err = s.db.ExecContext(
		ctx,
		queryCreateStack,
		user.GetTelegramID(),
		user.GetData(),
	)
	return
}
func (s *SqliteStorage) AddRecord(ctx context.Context, user model.User) error {
	foo, err := user.GetDataForNewRecord()
	if err != nil {
		return fmt.Errorf("error while getting user data for new record: %w", err)
	}
	_, err = s.db.ExecContext(
		ctx,
		queryAddRecord,
		foo.StackID,
		foo.RecTitle,
		foo.RecType,
		foo.RecStatus, //TODO update status
	)
	return err
}
func (s *SqliteStorage) UpdateRecordTitle(ctx context.Context, user model.User, recId int) (err error) {
	_, err = s.db.ExecContext(ctx, queryUpdateRecordTitle, user.GetData(), recId)
	return
}
func (s *SqliteStorage) UpdateRecordType(ctx context.Context, user model.User, recId int) (err error) {
	_, err = s.db.ExecContext(ctx, queryUpdateRecordType, user.GetData(), recId)
	return
}
func (s *SqliteStorage) UpdateRecordStatus(ctx context.Context, user model.User, recId int) (err error) {
	_, err = s.db.ExecContext(ctx, queryUpdateRecordStatus, user.GetData(), recId)
	return // TODO UPDATE STATUS
}
func (s *SqliteStorage) GetUserStackIDs(ctx context.Context, user model.User) (stacks []int, err error) {
	panic("unimplemented")
}
func (s *SqliteStorage) GetOwnedStacks(ctx context.Context, user model.User) (stacks []model.Stack, err error) {
	panic("unimplemented")
}
func (s *SqliteStorage) GetStack(ctx context.Context, stackID int64) (*model.Stack, error) {
	var foo model.Stack
	err := s.db.QueryRowContext(
		ctx,
		queryGetStackById,
		stackID,
	).Scan(
		&foo.ID,
		&foo.OwnerId,
		&foo.Title,
		&foo.Created,
		&foo.Updated,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &foo, nil
}
func (s *SqliteStorage) GetStackRecords(ctx context.Context, stackID int64) ([]model.Entry, error) {
	rows, err := s.db.QueryContext(ctx, queryGetStackRecords, stackID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("GetStackRows error while extracting: %v", err)
		}
	}(rows)

	var entries []model.Entry
	for rows.Next() {
		var e model.Entry
		if err := rows.Scan(
			&e.Id,
			&e.StackId,
			&e.Title,
			&e.Type,
			&e.Status,
			&e.Created,
			&e.Updated,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
func (s *SqliteStorage) UpdateRecent(ctx context.Context, user model.User) (err error) {
	_, err = s.db.ExecContext(ctx, queryUpdateRecent, user.GetData(), user.GetTelegramID())
	return
}
func (s *SqliteStorage) UpdateUserStacks(ctx context.Context, user model.User) error {
	panic("unimplemented")
}
func (s *SqliteStorage) Close() {
	_ = s.db.Close()
}
