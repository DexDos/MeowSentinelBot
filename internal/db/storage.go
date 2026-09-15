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
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

type Storage interface {
	initTables(context.Context) error
	AddUser(context.Context, model.User) error
	GetUser(context.Context, int64) (model.TelegramUser, error)
	CreateStack(context.Context, model.User) (int64, error)
	AddStackToUser(context.Context, *model.TelegramUser) error
	SetCurrentStack(ctx context.Context, user *model.TelegramUser) (err error)
	GetCurrentStack(ctx context.Context, user *model.TelegramUser) (stackID int64, err error)
	AddEntry(context.Context, model.User) (int64, error)
	UpdateEntryField(context.Context, int64, EntryField, string) error
	GetUserStacks(context.Context, model.User) ([]model.StackInfo, error)
	GetOwnedStacks(context.Context, model.User) ([]model.StackInfo, error)
	GetStackByID(context.Context, int64) (*model.StackInfo, error)
	GetStackRecords(context.Context, int64) ([]model.Entry, error)
	UpdateRecent(context.Context, model.User) error
	Transaction(context.Context, func(context.Context) error) error
	GetExecutor(context.Context) DBTX
	Close()
}

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type contextKey struct{}

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
	`CREATE TABLE IF NOT EXISTS selected_user_stacks (
    					user_id INTEGER PRIMARY KEY
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
		FROM stacks 
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
	queryGetUserByID = `
		SELECT
			telegram_id
			,recent_media
			,created_at
			,updated_at
		FROM users 
		WHERE telegram_id=?`
	queryGetCurrentUserStack = `
		SELECT
			stack_id
		FROM selected_user_stacks 
		WHERE user_id=?`
	queryAddUser = `
		INSERT INTO users(telegram_id)
					VALUES(?)
					ON CONFLICT(telegram_id) DO NOTHING;`
	queryCreateStack = `
		INSERT INTO stacks(owner_id, title)
		VALUES(?,?)`
	queryAddRecord = `
		INSERT INTO entries(
		                    stack_id
		                    ,title
		                    ,type
		                    ,current_season
		                    ,current_episode)
		VALUES(?,?,?,?,?)`
	queryAddStackToUser = `
		INSERT INTO user_stacks(user_id, stack_id)
		VALUES (?,?)`
	queryUpdateUserCurrentStack = `
		INSERT INTO selected_user_stacks(user_id, stack_id)
		VALUES (?,?) 
		ON CONFLICT (user_id) 
		DO UPDATE SET stack_id=EXCLUDED.stack_id;`
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
	queryUpdateEntryField = "UPDATE" + `entries SET %s=? WHERE id=?`
)

type EntryField string

const (
	EntryTitleField          EntryField = "title"
	EntryTypeField           EntryField = "type"
	EntryCurrentSeasonField  EntryField = "current_season"
	EntryCurrentEpisodeField EntryField = "current_episode"
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
	executor := s.GetExecutor(ctx)
	_, err = executor.ExecContext(
		ctx,
		queryAddUser,
		user.GetTelegramID(),
	)
	return
}
func (s *SqliteStorage) GetUser(ctx context.Context, userID int64) (user model.TelegramUser, err error) {
	panic("implement me")
}
func (s *SqliteStorage) CreateStack(ctx context.Context, user model.User) (id int64, err error) {
	executor := s.GetExecutor(ctx)
	res, err := executor.ExecContext(
		ctx,
		queryCreateStack,
		user.GetTelegramID(),
		user.GetPayload(),
	)
	if err != nil {
		return 0, fmt.Errorf("create stack error: %w", err)
	}
	return res.LastInsertId()
}
func (s *SqliteStorage) AddStackToUser(ctx context.Context, user *model.TelegramUser) (err error) {
	executor := s.GetExecutor(ctx)
	_, err = executor.ExecContext(
		ctx,
		queryAddStackToUser,
		user.GetTelegramID(),
		user.GetCurrentStackID(),
	)
	return
}
func (s *SqliteStorage) SetCurrentStack(ctx context.Context, user *model.TelegramUser) (err error) {
	executor := s.GetExecutor(ctx)
	_, err = executor.ExecContext(
		ctx,
		queryUpdateUserCurrentStack,
		user.User.ID,
		user.UserData.CurrentStackID,
	)
	return
}
func (s *SqliteStorage) GetCurrentStack(ctx context.Context, user *model.TelegramUser) (stackID int64, err error) {
	executor := s.GetExecutor(ctx)
	err = executor.QueryRowContext(
		ctx,
		queryGetCurrentUserStack,
		user.User.ID,
	).Scan(&stackID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return
}
func (s *SqliteStorage) AddEntry(ctx context.Context, user model.User) (entryID int64, err error) {
	foo := user.GetPayload()
	res, err := s.db.ExecContext(
		ctx,
		queryAddRecord,
		user.GetCurrentStackID(),
		foo,
		"—",
		0,
		0, //foo.RecStatus, //TODO update status
	)
	if err != nil {
		return 0, err
	}
	entryID, err = res.LastInsertId()
	return
}
func (s *SqliteStorage) UpdateEntryField(ctx context.Context, eID int64, f EntryField, rawValue string) (err error) {
	executor := s.GetExecutor(ctx)
	var parsedValue any

	switch f {
	case EntryTitleField, EntryTypeField:
		parsedValue = rawValue
	case EntryCurrentSeasonField, EntryCurrentEpisodeField:
		val, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer payload for %s: %w", f, err)
		}
		parsedValue = val
	default:
		return fmt.Errorf("forbidden or unknown column: %s", f)
	}

	query := fmt.Sprintf(queryUpdateEntryField, string(f))

	_, err = executor.ExecContext(
		ctx,
		query,
		parsedValue,
		eID,
	)
	return err
}

func (s *SqliteStorage) GetUserStacks(ctx context.Context, user model.User) ([]model.StackInfo, error) {

	rows, err := s.db.QueryContext(
		ctx,
		queryGetUserStacks,
		user.GetTelegramID(),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	defer func(*sql.Rows) {
		_ = rows.Close()
	}(rows)

	var stacks []model.StackInfo

	for rows.Next() {
		var stack model.StackInfo

		err = rows.Scan(
			&stack.ID,
			&stack.OwnerId,
			&stack.Title,
			&stack.Created,
			&stack.Updated,
		)

		if err != nil {
			return nil, fmt.Errorf("scan stack row error: %w", err)
		}

		stacks = append(stacks, stack)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return stacks, nil
}
func (s *SqliteStorage) GetOwnedStacks(ctx context.Context, user model.User) ([]model.StackInfo, error) {

	rows, err := s.db.QueryContext(
		ctx,
		queryGetOwnedStacks,
		user.GetTelegramID(),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	defer func(*sql.Rows) {
		_ = rows.Close()
	}(rows)

	var stacks []model.StackInfo

	for rows.Next() {
		var stack model.StackInfo

		err = rows.Scan(
			&stack.ID,
			&stack.OwnerId,
			&stack.Title,
			&stack.Created,
			&stack.Updated,
		)

		if err != nil {
			return nil, fmt.Errorf("scan stack row error: %w", err)
		}

		stacks = append(stacks, stack)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return stacks, nil
}
func (s *SqliteStorage) GetStackByID(ctx context.Context, stackID int64) (*model.StackInfo, error) {
	var foo model.StackInfo
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
	_, err = s.db.ExecContext(ctx, queryUpdateRecent, user.GetPayload(), user.GetTelegramID())
	return
}

func (s *SqliteStorage) WithContext(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, contextKey{}, tx)
}
func (s *SqliteStorage) GetExecutor(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(contextKey{}).(*sql.Tx); ok {
		return tx
	}
	return s.db
}
func (s *SqliteStorage) Transaction(ctx context.Context, fn func(txXtx context.Context) error) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	txCtx := s.WithContext(ctx, tx)
	err = fn(txCtx)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SqliteStorage) Close() {
	_ = s.db.Close()
}
