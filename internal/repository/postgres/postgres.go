package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mipecx/survey-bot-go/internal/models"
)

type Storage struct {
	Pool *pgxpool.Pool
}

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

func (s *Storage) Close() {
	s.Pool.Close()
}

func (s *Storage) GetOrCreateUser(ctx context.Context, tgID int64, username string) (*models.User, error) {
	var user models.User

	query := `
        INSERT INTO users (tg_id, username)
       VALUES ($1, $2)
        ON CONFLICT (tg_id) DO UPDATE SET username = EXCLUDED.username
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

func (s *Storage) GetStep(ctx context.Context, tgID int64) (string, error) {
	var step string
	query := `SELECT current_step FROM users WHERE tg_id = $1`

	err := s.Pool.QueryRow(ctx, query, tgID).Scan(&step)
	if err != nil {
		return "", fmt.Errorf("failed to get step: %w", err)
	}
	return step, nil
}

func (s *Storage) UpdateStep(ctx context.Context, tgID int64, step string) error {
	query := `UPDATE users SET current_step = $1 WHERE tg_id = $2`

	_, err := s.Pool.Exec(ctx, query, step, tgID)

	if err != nil {
		return fmt.Errorf("failed to update step: %w", err)
	}

	return nil
}

func (s *Storage) GetForm(ctx context.Context, tgID int64) (string, error) {
	var step string
	query := `SELECT current_form FROM users WHERE tg_id = $1`

	err := s.Pool.QueryRow(ctx, query, tgID).Scan(&step)
	if err != nil {
		return "", fmt.Errorf("failed to get form: %w", err)
	}
	return step, nil
}

func (s *Storage) UpdateForm(ctx context.Context, tgID int64, step string) error {
	query := `UPDATE users SET current_form = $1 WHERE tg_id = $2`

	_, err := s.Pool.Exec(ctx, query, step, tgID)

	if err != nil {
		return fmt.Errorf("failed to update step: %w", err)
	}

	return nil
}

func (s *Storage) ResetUserProgress(ctx context.Context, tgID int64, newForm string) error {
	query := `
		UPDATE users
		SET current_form = $1,
		    current_step = '',
		    survey_data = '{}'
		WHERE tg_id = $2
	`
	_, err := s.Pool.Exec(ctx, query, newForm, tgID)
	if err != nil {
		return fmt.Errorf("failed to reset progress: %w", err)
	}
	return nil
}

func (s *Storage) SaveAnswer(ctx context.Context, tgID int64, key string, value any) error {
	var query string
	var args []any

	valStr, _ := value.(string)

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
	return err
}
