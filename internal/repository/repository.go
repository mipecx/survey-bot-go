package repository

import (
	"context"

	"github.com/mipecx/survey-bot-go/internal/models"
)

type UserRepository interface {
	GetOrCreateUser(ctx context.Context, tgID int64, username string) (*models.User, error)
	GetStep(ctx context.Context, id int64) (string, error)
	UpdateStep(ctx context.Context, tgID int64, step string) error
	GetForm(ctx context.Context, id int64) (string, error)
	UpdateForm(ctx context.Context, tgID int64, step string) error
	ResetUserProgress(ctx context.Context, tgID int64, newForm string) error
	SaveAnswer(ctx context.Context, tgID int64, key string, value any) error
}
