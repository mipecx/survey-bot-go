// Package postgres implements the repository.UserRepository interface
// using a PostgreSQL database via the pgx/v5 connection pool.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mipecx/survey-bot-go/internal/ctxlog"
	"github.com/mipecx/survey-bot-go/internal/models"
)

// ErrInvalidValueType is returned by SaveAnswer when the value argument
// is not a string. All survey answers are expected to be string-typed.
var ErrInvalidValueType = errors.New("invalid value type in survey data")

// Storage wraps a pgxpool connection pool and implements repository.UserRepository.
// The logger field is used as a fallback when no per-request logger is in context.
type Storage struct {
	Pool   *pgxpool.Pool
	logger *slog.Logger
}

// Close gracefully closes the underlying connection pool.
// Safe to call on a nil Pool.
func (r *Storage) Close() error {
	if r.Pool != nil {
		r.Pool.Close()
	}
	return nil
}

// New creates a new pgxpool connection pool, verifies connectivity via Ping,
// and returns a ready-to-use Storage. The provided logger is used for
// startup errors and as a fallback in methods where no context logger is set.
func New(ctx context.Context, connString string, logger *slog.Logger) (*Storage, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		logger.Error("Database: unable to create connection pool", "error", err)
		return nil, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		logger.Error("Database: ping failed", "error", err)
		return nil, err
	}

	return &Storage{Pool: pool, logger: logger}, nil
}

// GetOrCreateUser upserts the user record for tgID.
// On conflict, username is updated only if the new value is non-empty.
// Returns the full current user record including profile and survey state fields.
func (s *Storage) GetOrCreateUser(ctx context.Context, tgID int64, username string) (*models.User, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	var user models.User

	query := `
		INSERT INTO users (tg_id, username)
		VALUES ($1, $2)
		ON CONFLICT (tg_id) DO UPDATE
		SET username = COALESCE(NULLIF($2, ''), users.username)
		RETURNING tg_id, username, current_form, current_step, full_name, phone, birth_date, pending_form, city, gender, created_at;
	`

	err := s.Pool.QueryRow(ctx, query, tgID, username).Scan(
		&user.TGID,
		&user.Username,
		&user.CurrentForm,
		&user.CurrentStep,
		&user.FullName,
		&user.Phone,
		&user.BirthDate,
		&user.PendingForm,
		&user.City,
		&user.Gender,
		&user.CreatedAt,
	)

	if err != nil {
		logger.Error("Database: failed to get or create user", "error", err)
		return nil, err
	}

	return &user, nil
}

// GetStep returns the current survey step ID for the given user.
func (s *Storage) GetStep(ctx context.Context, tgID int64) (string, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	var step string
	query := `SELECT current_step FROM users WHERE tg_id = $1`

	err := s.Pool.QueryRow(ctx, query, tgID).Scan(&step)
	if err != nil {
		logger.Error("Database: failed to get step", "error", err)
		return "", err
	}
	return step, nil
}

// UpdateStep sets the current survey step ID for the given user.
func (s *Storage) UpdateStep(ctx context.Context, tgID int64, step string) error {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	query := `UPDATE users SET current_step = $1 WHERE tg_id = $2`

	_, err := s.Pool.Exec(ctx, query, step, tgID)

	if err != nil {
		logger.Error("Database: failed to update step", "error", err)
		return err
	}

	return nil
}

// GetForm returns the current form name for the given user.
func (s *Storage) GetForm(ctx context.Context, tgID int64) (string, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	var step string
	query := `SELECT current_form FROM users WHERE tg_id = $1`

	err := s.Pool.QueryRow(ctx, query, tgID).Scan(&step)
	if err != nil {
		logger.Error("Database: failed to get form", "error", err)
		return "", err
	}
	return step, nil
}

// UpdateForm sets the current form name for the given user.
func (s *Storage) UpdateForm(ctx context.Context, tgID int64, form string) error {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	query := `UPDATE users SET current_form = $1 WHERE tg_id = $2`

	_, err := s.Pool.Exec(ctx, query, form, tgID)

	if err != nil {
		logger.Error("Database: failed to update form", "error", err)
		return err
	}

	return nil
}

// ResetUserProgress resets the user's form, step to their initial state.
func (s *Storage) ResetUserProgress(ctx context.Context, tgID int64, form string) error {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	query := `
		UPDATE users
		SET current_form = $1,
		    current_step = ''
		WHERE tg_id = $2
	`
	_, err := s.Pool.Exec(ctx, query, form, tgID)
	if err != nil {
		logger.Error("Database: failed to reset progress", "error", err)
		return err
	}
	return nil
}

// SaveAnswer persist a single survey answer for the given user.
// Structured fields (full_name, phone, birth_date, city, gender) are saved to dedicated columns;
// all other answers are merged into the survey_data JSONB column.
// Returns ErrInvalidValueType if value is not a string.
func (s *Storage) SaveAnswer(ctx context.Context, tgID int64, key string, value any) error {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	var query string
	var args []any

	valStr, ok := value.(string)
	if !ok {
		logger.Error("Database: type mismatch in survey data",
			"key", key,
			"expected", "string",
			"got_type", fmt.Sprintf("%T", value))
		return ErrInvalidValueType
	}

	switch key {
	case "reg_name":
		query = `UPDATE users SET full_name = $1 WHERE tg_id = $2`
		args = []any{value, tgID}
	case "reg_phone":
		query = `UPDATE users SET phone = $1 WHERE tg_id = $2`
		args = []any{value, tgID}
	case "reg_birthdate":
		parsedDate, err := time.Parse("02.01.2006", valStr)
		if err != nil {
			logger.Error("Database: level date parse failed", "error", err)
			return err
		}
		query = `UPDATE users SET birth_date = $1 WHERE tg_id = $2`
		args = []any{parsedDate, tgID}
	case "reg_city":
		query = `UPDATE users SET city = $1 WHERE tg_id = $2`
		args = []any{value, tgID}
	case "reg_gender":
		query = `UPDATE users SET gender = $1 WHERE tg_id = $2`
		args = []any{value, tgID}
	default:
		query = `
            UPDATE users
            SET survey_data = survey_data || jsonb_build_object($1::text, to_jsonb($2::text))
            WHERE tg_id = $3`
		args = []any{key, value, tgID}
	}

	_, err := s.Pool.Exec(ctx, query, args...)
	if err != nil {
		logger.Error("Database: exec failed",
			"query_type", key,
			"error", err)
		return fmt.Errorf("db: failed to save %s: %w", key, err)
	}
	return nil
}

// GetAnswersByForm returns all survey answers for the user as a flat string map.
// Profile columns (full_name, phone, birth_date, city, gender) are merged with
// the survey_data JSONB field. Keys match the question IDs used in AllForms.
func (s *Storage) GetAnswersByForm(ctx context.Context, tgID int64) (map[string]string, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	var (
		fullName  *string
		phone     *string
		birthDate *time.Time
		city      *string
		gender    *string
		jsonData  []byte
	)

	query := `
		SELECT full_name, phone, birth_date, city, gender, survey_data
		FROM users
		WHERE tg_id = $1
	`
	err := s.Pool.QueryRow(ctx, query, tgID).Scan(&fullName, &phone, &birthDate, &city, &gender, &jsonData)
	if err != nil {
		logger.Error("Database: failed to fetch answers", "error", err)
		return nil, err
	}

	answers := make(map[string]string)

	if len(jsonData) > 0 {
		if err := json.Unmarshal(jsonData, &answers); err != nil {
			logger.Warn("Database: failed to unmarshal survey_data, starting with empty map",
				"error", err)
		}
	}
	if fullName != nil {
		answers["reg_name"] = *fullName
	}
	if phone != nil {
		answers["reg_phone"] = *phone
	}
	if birthDate != nil {
		answers["reg_birthdate"] = birthDate.Format("02.01.2006")
	}
	if city != nil {
		answers["reg_city"] = *city
	}
	if gender != nil {
		answers["reg_gender"] = *gender
	}
	return answers, nil
}

// SetPendingForm stores the form the user wanted to access before being
// redirected to contact data collection.
func (s *Storage) SetPendingForm(ctx context.Context, tgID int64, form string) error {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	_, err := s.Pool.Exec(ctx, `UPDATE users SET pending_form = $1 WHERE tg_id = $2`, form, tgID)
	if err != nil {
		logger.Error("Database: failed to set pending_form", "error", err)
	}
	return err
}

// ClearPendingForm removes the pending form redirect after it has been consumed.
func (s *Storage) ClearPendingForm(ctx context.Context, tgID int64) error {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	_, err := s.Pool.Exec(ctx, `UPDATE users SET pending_form = NULL WHERE tg_id = $1`, tgID)
	if err != nil {
		logger.Error("Database: failed to clear pending form", "error", err)
	}
	return err
}

// GetAllUserIDs returns the Telegram user IDs of all registered users.
// Used by Broadcast to enumerate delivery targets.
func (s *Storage) GetAllUserIDs(ctx context.Context) ([]int64, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	rows, err := s.Pool.Query(ctx, `SELECT tg_id FROM users`)
	if err != nil {
		logger.Error("database: failed to get user ids", "error", err)
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			logger.Error("database: failed to scan user id", "error", err)
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
