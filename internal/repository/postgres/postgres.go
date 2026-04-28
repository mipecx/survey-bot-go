package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mipecx/survey-bot-go/internal/models"
)

// Storage wraps a pgxpool connection pool and implements userRepository.
type Storage struct {
	Pool *pgxpool.Pool
}

// Close gracefully closes the underlying connection pool.
func (r *Storage) Close() error {
	if r.Pool != nil {
		r.Pool.Close()
	}
	return nil
}

// New creates a new PostgreSQL connection pool and verifies connectivity via ping.
// Returns an error if the pool can not be created or database in unreachable.
func New(ctx context.Context, connString string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return &Storage{Pool: pool}, nil
}

// GetOrCreateUser inserts a new user or updates the username on conflict,
// returning the current user record.
func (s *Storage) GetOrCreateUser(ctx context.Context, tgID int64, username string) (*models.User, error) {
	var user models.User

	query := `
		INSERT INTO users (tg_id, username)
		VALUES ($1, $2)
		ON CONFLICT (tg_id) DO UPDATE
		SET username = COALESCE(NULLIF($2, ''), users.username)
		RETURNING tg_id, username, current_form, current_step, full_name, phone, birth_date, created_at;
	`

	err := s.Pool.QueryRow(ctx, query, tgID, username).Scan(
		&user.TGID,
		&user.Username,
		&user.CurrentForm,
		&user.CurrentStep,
		&user.FullName,
		&user.Phone,
		&user.BirthDate,
		&user.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get or create user: %w", err)
	}

	return &user, nil
}

// GetStep returns the current survey step ID for the given user.
func (s *Storage) GetStep(ctx context.Context, tgID int64) (string, error) {
	var step string
	query := `SELECT current_step FROM users WHERE tg_id = $1`

	err := s.Pool.QueryRow(ctx, query, tgID).Scan(&step)
	if err != nil {
		return "", fmt.Errorf("failed to get step: %w", err)
	}
	return step, nil
}

// UpdateStep sets the current survey step ID for the given user.
func (s *Storage) UpdateStep(ctx context.Context, tgID int64, step string) error {
	query := `UPDATE users SET current_step = $1 WHERE tg_id = $2`

	_, err := s.Pool.Exec(ctx, query, step, tgID)

	if err != nil {
		return fmt.Errorf("failed to update step: %w", err)
	}

	return nil
}

// GetForm returns the current form name for the given user.
func (s *Storage) GetForm(ctx context.Context, tgID int64) (string, error) {
	var step string
	query := `SELECT current_form FROM users WHERE tg_id = $1`

	err := s.Pool.QueryRow(ctx, query, tgID).Scan(&step)
	if err != nil {
		return "", fmt.Errorf("failed to get form: %w", err)
	}
	return step, nil
}

// UpdateForm sets the current form name for the given user.
func (s *Storage) UpdateForm(ctx context.Context, tgID int64, form string) error {
	query := `UPDATE users SET current_form = $1 WHERE tg_id = $2`

	_, err := s.Pool.Exec(ctx, query, form, tgID)

	if err != nil {
		return fmt.Errorf("failed to update form: %w", err)
	}

	return nil
}

// ResetUserProgress resets the user's form, step, and survey_data to their initial state.
func (s *Storage) ResetUserProgress(ctx context.Context, tgID int64, form string) error {
	query := `
		UPDATE users
		SET current_form = $1,
		    current_step = ''
		WHERE tg_id = $2
	`
	_, err := s.Pool.Exec(ctx, query, form, tgID)
	if err != nil {
		return fmt.Errorf("failed to reset progress: %w", err)
	}
	return nil
}

// SaveAnswer persist a single survey answer for the given user.
// Structured fields (full_name, phone, birth_date) are saved to dedicated columns;
// all other answers are merged into the survey_data JSONB column.
func (s *Storage) SaveAnswer(ctx context.Context, tgID int64, key string, value any) error {
	var query string
	var args []any

	valStr, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string value for key %s, got %T", key, value)
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
			return fmt.Errorf("database level date parse failed: %w", err)
		}
		query = `UPDATE users SET birth_date = $1 WHERE tg_id = $2`
		args = []any{parsedDate, tgID}
	default:
		query = `
            UPDATE users
            SET survey_data = survey_data || jsonb_build_object($1::text, to_jsonb($2::text))
            WHERE tg_id = $3`
		args = []any{key, value, tgID}
	}

	_, err := s.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to save answer (key=%s, tgID=%d): %w", key, tgID, err)
	}
	return nil
}

func (s *Storage) GetAnswersByForm(ctx context.Context, tgID int64) (map[string]string, error) {
	var (
		fullName  *string
		phone     *string
		birthDate *time.Time
		jsonData  []byte
	)

	query := `
		SELECT full_name, phone, birth_date, survey_data
		FROM users
		WHERE tg_id = $1
	`
	err := s.Pool.QueryRow(ctx, query, tgID).Scan(&fullName, &phone, &birthDate, &jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch answers: %w", err)
	}

	answers := make(map[string]string)

	if len(jsonData) > 0 {
		if err := json.Unmarshal(jsonData, &answers); err != nil {
			fmt.Printf("warning: failed to unmarshal survey_data: %v\n", err)
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
	return answers, nil
}
