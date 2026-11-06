package sqlite

import (
	"MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/model"
	"MeowSentinelBot/internal/storage"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type txKey struct{}

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Storage struct {
	db *sql.DB
}

var _ storage.Storage = (*Storage)(nil)

func NewStorage(cfg *config.DB) (*Storage, error) {
	dsn := fmt.Sprintf(cfg.DSN, cfg.Path)
	db, err := sql.Open(cfg.Controller, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db error: %w", err)
	}

	db.SetConnMaxLifetime(time.Hour)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db error: %w", err)
	}

	s := &Storage{db: db}
	if err := s.initTables(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init tables error: %w", err)
	}

	return s, nil
}

func (s *Storage) getExecutor(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return s.db
}

func (s *Storage) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) initTables(ctx context.Context) error {
	ddl := []string{
		`PRAGMA foreign_keys = ON;`,

		`CREATE TABLE IF NOT EXISTS users (
			telegram_id INTEGER PRIMARY KEY,
			username TEXT NOT NULL DEFAULT '',
			first_name TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS stacks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES users(telegram_id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS user_stacks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			stack_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(telegram_id) ON DELETE CASCADE,
			FOREIGN KEY (stack_id) REFERENCES stacks(id) ON DELETE CASCADE,
			UNIQUE(user_id, stack_id)
		);`,

		`CREATE TABLE IF NOT EXISTS stack_invites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			stack_id INTEGER NOT NULL,
			created_by INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			is_used INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (stack_id) REFERENCES stacks(id) ON DELETE CASCADE,
			FOREIGN KEY (created_by) REFERENCES users(telegram_id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			stack_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'Movie',
			current_season INTEGER NOT NULL DEFAULT 1,
			current_episode INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (stack_id) REFERENCES stacks(id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS entry_ratings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			entry_id INTEGER NOT NULL,
			rating INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(telegram_id) ON DELETE CASCADE,
			FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE,
			UNIQUE(user_id, entry_id)
		);`,

		`CREATE TABLE IF NOT EXISTS selected_user_stacks (
			user_id INTEGER PRIMARY KEY,
			stack_id INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(telegram_id) ON DELETE CASCADE,
			FOREIGN KEY (stack_id) REFERENCES stacks(id) ON DELETE CASCADE
		);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_stacks_user_stack ON user_stacks(user_id, stack_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_stacks_user ON user_stacks(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_stack ON entries(stack_id);`,
		`CREATE INDEX IF NOT EXISTS idx_stack_invites_code ON stack_invites(code);`,
		`CREATE INDEX IF NOT EXISTS idx_entry_ratings_entry ON entry_ratings(entry_id);`,
	}

	for _, stmt := range ddl {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec schema stmt failed: %w (stmt: %s)", err, stmt)
		}
	}

	// Safe migrations for preexisting databases
	migrations := []string{
		`ALTER TABLE users ADD COLUMN username TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE users ADD COLUMN first_name TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE user_stacks ADD COLUMN role TEXT NOT NULL DEFAULT 'member';`,
		`DELETE FROM user_stacks WHERE id NOT IN (SELECT MIN(id) FROM user_stacks GROUP BY user_id, stack_id);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_stacks_user_stack ON user_stacks(user_id, stack_id);`,
	}
	for _, m := range migrations {
		// Ignore errors if columns already exist
		_, _ = s.db.ExecContext(ctx, m)
	}

	return nil
}

// =========================================================================
// Users
// =========================================================================

func (s *Storage) EnsureUser(ctx context.Context, u model.User) error {
	exec := s.getExecutor(ctx)
	query := `
		INSERT INTO users (telegram_id, username, first_name)
		VALUES (?, ?, ?)
		ON CONFLICT(telegram_id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			updated_at = CURRENT_TIMESTAMP;`
	_, err := exec.ExecContext(ctx, query, u.TelegramID, u.Username, u.FirstName)
	return err
}

func (s *Storage) GetUser(ctx context.Context, userID int64) (*model.User, error) {
	exec := s.getExecutor(ctx)
	query := `SELECT telegram_id, username, first_name, created_at, updated_at FROM users WHERE telegram_id = ?`
	var u model.User
	err := exec.QueryRowContext(ctx, query, userID).Scan(
		&u.TelegramID,
		&u.Username,
		&u.FirstName,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// =========================================================================
// Stacks
// =========================================================================

func (s *Storage) CreateStack(ctx context.Context, ownerID int64, title string) (int64, error) {
	var stackID int64
	err := s.Transaction(ctx, func(txCtx context.Context) error {
		exec := s.getExecutor(txCtx)

		// 1. Insert into stacks
		res, err := exec.ExecContext(txCtx, `INSERT INTO stacks (owner_id, title) VALUES (?, ?)`, ownerID, title)
		if err != nil {
			return fmt.Errorf("insert stack: %w", err)
		}
		stackID, err = res.LastInsertId()
		if err != nil {
			return err
		}

		// 2. Link in user_stacks with role 'owner'
		_, err = exec.ExecContext(txCtx, `
			INSERT OR REPLACE INTO user_stacks (user_id, stack_id, role)
			VALUES (?, ?, ?);`,
			ownerID, stackID, model.RoleOwner)
		if err != nil {
			return fmt.Errorf("link owner in user_stacks: %w", err)
		}

		// 3. Mark as selected stack
		_, err = exec.ExecContext(txCtx, `
			INSERT INTO selected_user_stacks (user_id, stack_id)
			VALUES (?, ?)
			ON CONFLICT(user_id) DO UPDATE SET stack_id = EXCLUDED.stack_id;`,
			ownerID, stackID)
		if err != nil {
			return fmt.Errorf("set selected stack: %w", err)
		}

		return nil
	})

	return stackID, err
}

func (s *Storage) GetStackByID(ctx context.Context, stackID int64) (*model.StackInfo, error) {
	exec := s.getExecutor(ctx)
	query := `
		SELECT 
			s.id,
			s.owner_id,
			s.title,
			(SELECT COUNT(*) FROM entries e WHERE e.stack_id = s.id) AS entries_count,
			s.created_at,
			s.updated_at
		FROM stacks s
		WHERE s.id = ?;`
	var si model.StackInfo
	err := exec.QueryRowContext(ctx, query, stackID).Scan(
		&si.ID,
		&si.OwnerID,
		&si.Title,
		&si.EntriesCount,
		&si.CreatedAt,
		&si.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &si, nil
}

func (s *Storage) GetUserStacks(ctx context.Context, userID int64) ([]model.StackInfo, error) {
	exec := s.getExecutor(ctx)
	query := `
		SELECT 
			s.id,
			s.owner_id,
			s.title,
			us.role,
			(SELECT COUNT(*) FROM entries e WHERE e.stack_id = s.id) AS entries_count,
			s.created_at,
			s.updated_at
		FROM stacks s
		JOIN user_stacks us ON us.stack_id = s.id
		WHERE us.user_id = ?
		ORDER BY s.updated_at DESC;`
	rows, err := exec.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stacks []model.StackInfo
	for rows.Next() {
		var si model.StackInfo
		if err := rows.Scan(
			&si.ID,
			&si.OwnerID,
			&si.Title,
			&si.Role,
			&si.EntriesCount,
			&si.CreatedAt,
			&si.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stack info: %w", err)
		}
		stacks = append(stacks, si)
	}
	return stacks, rows.Err()
}

func (s *Storage) UpdateStackTitle(ctx context.Context, stackID int64, title string) error {
	exec := s.getExecutor(ctx)
	query := `UPDATE stacks SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`
	_, err := exec.ExecContext(ctx, query, title, stackID)
	return err
}

func (s *Storage) DeleteStack(ctx context.Context, stackID int64) error {
	exec := s.getExecutor(ctx)
	query := `DELETE FROM stacks WHERE id = ?;`
	_, err := exec.ExecContext(ctx, query, stackID)
	return err
}

func (s *Storage) LeaveStack(ctx context.Context, userID, stackID int64) error {
	return s.Transaction(ctx, func(txCtx context.Context) error {
		exec := s.getExecutor(txCtx)
		_, err := exec.ExecContext(txCtx, `DELETE FROM user_stacks WHERE user_id = ? AND stack_id = ?;`, userID, stackID)
		if err != nil {
			return err
		}
		_, _ = exec.ExecContext(txCtx, `DELETE FROM selected_user_stacks WHERE user_id = ? AND stack_id = ?;`, userID, stackID)
		return nil
	})
}

func (s *Storage) SetSelectedStack(ctx context.Context, userID, stackID int64) error {
	exec := s.getExecutor(ctx)
	query := `
		INSERT INTO selected_user_stacks (user_id, stack_id)
		VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET stack_id = EXCLUDED.stack_id;`
	_, err := exec.ExecContext(ctx, query, userID, stackID)
	return err
}

func (s *Storage) GetSelectedStackID(ctx context.Context, userID int64) (int64, error) {
	exec := s.getExecutor(ctx)
	query := `SELECT stack_id FROM selected_user_stacks WHERE user_id = ?;`
	var stackID int64
	err := exec.QueryRowContext(ctx, query, userID).Scan(&stackID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return stackID, nil
}

func (s *Storage) IsStackMember(ctx context.Context, userID, stackID int64) (bool, string, error) {
	exec := s.getExecutor(ctx)
	query := `SELECT role FROM user_stacks WHERE user_id = ? AND stack_id = ?;`
	var role string
	err := exec.QueryRowContext(ctx, query, userID, stackID).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, role, nil
}

// =========================================================================
// Invites
// =========================================================================

func (s *Storage) CreateInvite(ctx context.Context, inv model.Invite) error {
	exec := s.getExecutor(ctx)
	query := `
		INSERT INTO stack_invites (code, stack_id, created_by, expires_at, is_used)
		VALUES (?, ?, ?, ?, ?);`
	_, err := exec.ExecContext(ctx, query, inv.Code, inv.StackID, inv.CreatedBy, inv.ExpiresAt, 0)
	return err
}

func (s *Storage) GetInviteByCode(ctx context.Context, code string) (*model.Invite, error) {
	exec := s.getExecutor(ctx)
	query := `
		SELECT id, code, stack_id, created_by, expires_at, is_used, created_at
		FROM stack_invites
		WHERE code = ?;`
	var inv model.Invite
	var isUsedInt int
	err := exec.QueryRowContext(ctx, query, code).Scan(
		&inv.ID,
		&inv.Code,
		&inv.StackID,
		&inv.CreatedBy,
		&inv.ExpiresAt,
		&isUsedInt,
		&inv.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	inv.IsUsed = isUsedInt == 1
	return &inv, nil
}

func (s *Storage) ConsumeInvite(ctx context.Context, code string, userID int64) (*model.StackInfo, error) {
	var stack *model.StackInfo

	err := s.Transaction(ctx, func(txCtx context.Context) error {
		exec := s.getExecutor(txCtx)

		// 1. Fetch valid invite
		var inv model.Invite
		var isUsedInt int
		query := `
			SELECT id, code, stack_id, created_by, expires_at, is_used, created_at
			FROM stack_invites
			WHERE code = ?;`
		err := exec.QueryRowContext(txCtx, query, code).Scan(
			&inv.ID,
			&inv.Code,
			&inv.StackID,
			&inv.CreatedBy,
			&inv.ExpiresAt,
			&isUsedInt,
			&inv.CreatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errors.New("invite code not found")
			}
			return err
		}
		if isUsedInt == 1 {
			return errors.New("invite code has already been used")
		}
		if time.Now().After(inv.ExpiresAt) {
			return errors.New("invite code has expired")
		}

		// 2. Mark code as used
		_, err = exec.ExecContext(txCtx, `UPDATE stack_invites SET is_used = 1 WHERE id = ?;`, inv.ID)
		if err != nil {
			return fmt.Errorf("mark invite used: %w", err)
		}

		// 3. Add user to user_stacks
		_, err = exec.ExecContext(txCtx, `
			INSERT OR IGNORE INTO user_stacks (user_id, stack_id, role)
			VALUES (?, ?, ?);`,
			userID, inv.StackID, model.RoleMember)
		if err != nil {
			return fmt.Errorf("join stack: %w", err)
		}

		// 4. Set as selected stack
		_, err = exec.ExecContext(txCtx, `
			INSERT INTO selected_user_stacks (user_id, stack_id)
			VALUES (?, ?)
			ON CONFLICT(user_id) DO UPDATE SET stack_id = EXCLUDED.stack_id;`,
			userID, inv.StackID)
		if err != nil {
			return fmt.Errorf("set selected stack: %w", err)
		}

		// 5. Load stack info
		stack, err = s.GetStackByID(txCtx, inv.StackID)
		return err
	})

	return stack, err
}

// =========================================================================
// Entries
// =========================================================================

func (s *Storage) AddEntry(ctx context.Context, e model.Entry) (int64, error) {
	exec := s.getExecutor(ctx)
	query := `
		INSERT INTO entries (stack_id, title, type, current_season, current_episode)
		VALUES (?, ?, ?, ?, ?);`
	season := e.Season
	if season <= 0 {
		season = 1
	}
	ep := e.Episode
	if ep <= 0 {
		ep = 1
	}
	mediaType := e.Type
	if mediaType == "" {
		mediaType = "Movie"
	}
	res, err := exec.ExecContext(ctx, query, e.StackID, e.Title, mediaType, season, ep)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, _ = exec.ExecContext(ctx, `UPDATE stacks SET updated_at = CURRENT_TIMESTAMP WHERE id = ?;`, e.StackID)
	return id, nil
}

func (s *Storage) GetEntryByID(ctx context.Context, entryID int64) (*model.Entry, error) {
	exec := s.getExecutor(ctx)
	query := `
		SELECT 
			e.id,
			e.stack_id,
			e.title,
			e.type,
			e.current_season,
			e.current_episode,
			COALESCE(er.avg_rating, 0) AS avg_rating,
			COALESCE(er.rating_count, 0) AS rating_count,
			e.created_at,
			e.updated_at
		FROM entries e
		LEFT JOIN (
			SELECT 
				entry_id, 
				CAST(ROUND(AVG(rating)) AS INTEGER) AS avg_rating, 
				COUNT(*) AS rating_count 
			FROM entry_ratings 
			GROUP BY entry_id
		) er ON e.id = er.entry_id
		WHERE e.id = ?;`
	var (
		e       model.Entry
		rawRate int64
	)
	err := exec.QueryRowContext(ctx, query, entryID).Scan(
		&e.ID,
		&e.StackID,
		&e.Title,
		&e.Type,
		&e.Season,
		&e.Episode,
		&rawRate,
		&e.RatingCount,
		&e.CreatedAt,
		&e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	e.Rating = model.ScoreFromInt(rawRate)
	return &e, nil
}

func (s *Storage) GetStackEntries(ctx context.Context, stackID int64) ([]model.Entry, error) {
	exec := s.getExecutor(ctx)
	query := `
		SELECT 
			e.id,
			e.stack_id,
			e.title,
			e.type,
			e.current_season,
			e.current_episode,
			COALESCE(er.avg_rating, 0) AS avg_rating,
			COALESCE(er.rating_count, 0) AS rating_count,
			e.created_at,
			e.updated_at
		FROM entries e
		LEFT JOIN (
			SELECT 
				entry_id, 
				CAST(ROUND(AVG(rating)) AS INTEGER) AS avg_rating, 
				COUNT(*) AS rating_count 
			FROM entry_ratings 
			GROUP BY entry_id
		) er ON e.id = er.entry_id
		WHERE e.stack_id = ?
		ORDER BY e.type ASC, e.updated_at DESC;`
	rows, err := exec.QueryContext(ctx, query, stackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.Entry
	for rows.Next() {
		var (
			e       model.Entry
			rawRate int64
		)
		if err := rows.Scan(
			&e.ID,
			&e.StackID,
			&e.Title,
			&e.Type,
			&e.Season,
			&e.Episode,
			&rawRate,
			&e.RatingCount,
			&e.CreatedAt,
			&e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan entry row: %w", err)
		}
		e.Rating = model.ScoreFromInt(rawRate)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Storage) UpdateEntryTitle(ctx context.Context, entryID int64, title string) error {
	exec := s.getExecutor(ctx)
	query := `UPDATE entries SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`
	_, err := exec.ExecContext(ctx, query, title, entryID)
	return err
}

func (s *Storage) UpdateEntryType(ctx context.Context, entryID int64, mediaType string) error {
	exec := s.getExecutor(ctx)
	query := `UPDATE entries SET type = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`
	_, err := exec.ExecContext(ctx, query, mediaType, entryID)
	return err
}

func (s *Storage) UpdateEntryProgress(ctx context.Context, entryID int64, season, episode int64) error {
	exec := s.getExecutor(ctx)
	query := `UPDATE entries SET current_season = ?, current_episode = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`
	_, err := exec.ExecContext(ctx, query, season, episode, entryID)
	return err
}

func (s *Storage) DeleteEntry(ctx context.Context, entryID int64) error {
	exec := s.getExecutor(ctx)
	query := `DELETE FROM entries WHERE id = ?;`
	_, err := exec.ExecContext(ctx, query, entryID)
	return err
}

// =========================================================================
// Ratings
// =========================================================================

func (s *Storage) RateEntry(ctx context.Context, userID, entryID int64, score model.Score) error {
	exec := s.getExecutor(ctx)
	query := `
		INSERT INTO entry_ratings (user_id, entry_id, rating)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, entry_id) DO UPDATE SET rating = EXCLUDED.rating;`
	_, err := exec.ExecContext(ctx, query, userID, entryID, score.Int())
	return err
}

func (s *Storage) GetUserRating(ctx context.Context, userID, entryID int64) (model.Score, error) {
	exec := s.getExecutor(ctx)
	query := `SELECT rating FROM entry_ratings WHERE user_id = ? AND entry_id = ?;`
	var scoreInt int64
	err := exec.QueryRowContext(ctx, query, userID, entryID).Scan(&scoreInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.NotSeen, nil
		}
		return model.NotSeen, err
	}
	return model.ScoreFromInt(scoreInt), nil
}
