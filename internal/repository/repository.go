// Package repository defines the persistence interfaces for the survey bot.
// The concrete implementation lives in repository/postgres.
package repository

import (
	"context"

	"github.com/mipecx/survey-bot-go/internal/models"
)

// UserRepository defines the persistence contract for user state and survey answers.
// All methods accept a context for cancellation, timeout, and request-scoped logging.
type UserRepository interface {
	// GetOrCreateUser returns the existing user record for tgID, or inserts a new one.
	// On conflict, the username is updated if the new value is non-empty.
	GetOrCreateUser(ctx context.Context, tgID int64, username string) (*models.User, error)

	// GetStep returns the current survey step ID for the given user.
	GetStep(ctx context.Context, tgID int64) (string, error)

	// UpdateStep sets the current survey step ID for the given user.
	UpdateStep(ctx context.Context, tgID int64, step string) error

	// GetForm returns the current form ID for the given user.
	GetForm(ctx context.Context, tgID int64) (string, error)

	// UpdateForm sets the current form ID for the given user.
	UpdateForm(ctx context.Context, tgID int64, form string) error

	// ResetUserProgress resets the user's current form and clears the current step.
	// Pass FormMainMenu to return the user to the main menu state.
	ResetUserProgress(ctx context.Context, tgID int64, form string) error

	// SaveAnswer persists a single survey answer for the given user.
	// Structured fields (full_name, phone, birth_date, city, gender) are saved
	// to dedicated columns; all other answers are merged into the survey_data JSONB column.
	SaveAnswer(ctx context.Context, tgID int64, key string, value any) error

	// Structured fields (full_name, phone, birth_date, city, gender) are saved
	// to dedicated columns; all other answers are merged into the survey_data JSONB column.
	GetAnswersByForm(ctx context.Context, tgID int64) (map[string]string, error)

	// SetPendingForm stores the form the user intended to access before being
	// redirected to ContactForm for contact data collection.
	SetPendingForm(ctx context.Context, tgID int64, form string) error

	// ClearPendingForm removes the pending form redirect after it has been consumed.
	ClearPendingForm(ctx context.Context, tgID int64) error
}
