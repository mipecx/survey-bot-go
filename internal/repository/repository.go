package repository

import (
	"context"

	"github.com/mipecx/survey-bot-go/internal/models"
)

// UserRepository defines the persistence contract for user survey state and answers.
type UserRepository interface {
	GetOrCreateUser(ctx context.Context, tgID int64, username string) (*models.User, error)
	GetStep(ctx context.Context, tgID int64) (string, error)
	UpdateStep(ctx context.Context, tgID int64, step string) error
	GetForm(ctx context.Context, tgID int64) (string, error)
	UpdateForm(ctx context.Context, tgID int64, form string) error
	ResetUserProgress(ctx context.Context, tgID int64, form string) error
	SaveAnswer(ctx context.Context, tgID int64, key string, value any) error
	GetAnswersByForm(ctx context.Context, tgID int64) (map[string]string, error)
}
