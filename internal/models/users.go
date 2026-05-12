// Package models defines the domain types shared across the application layers.
package models

import (
	"time"
)

// User represents a Telegram bot user and their current survey state.
// Nullable fields use pointer types to distinguish between "not provided" and empty string.
type User struct {
	TGID        int64          `db:"tg_id"`
	Username    string         `db:"username"`
	CurrentForm string         `db:"current_form"`
	CurrentStep string         `db:"current_step"`
	FullName    *string        `db:"full_name"`
	Phone       *string        `db:"phone"`
	BirthDate   *time.Time     `db:"birth_date"`
	SurveyData  map[string]any `db:"survey_data"`
	PendingForm *string        `db:"pending_form"`
	CreatedAt   time.Time      `db:"created_at"`
	City        *string        `db:"city"`
	Gender      *string        `db:"gender"`
}
