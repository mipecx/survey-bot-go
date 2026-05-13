package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/mipecx/survey-bot-go/internal/models"
)

type repoStub struct {
	getAllUserIDsFn func(ctx context.Context) ([]int64, error)
}

func (r *repoStub) GetOrCreateUser(context.Context, int64, string) (*models.User, error) {
	return nil, nil
}
func (r *repoStub) GetStep(context.Context, int64) (string, error)         { return "", nil }
func (r *repoStub) UpdateStep(context.Context, int64, string) error        { return nil }
func (r *repoStub) GetForm(context.Context, int64) (string, error)         { return "", nil }
func (r *repoStub) UpdateForm(context.Context, int64, string) error        { return nil }
func (r *repoStub) ResetUserProgress(context.Context, int64, string) error { return nil }
func (r *repoStub) SaveAnswer(context.Context, int64, string, any) error   { return nil }
func (r *repoStub) GetAnswersByForm(context.Context, int64) (map[string]string, error) {
	return nil, nil
}
func (r *repoStub) SetPendingForm(context.Context, int64, string) error { return nil }
func (r *repoStub) ClearPendingForm(context.Context, int64) error       { return nil }
func (r *repoStub) GetAllUserIDs(ctx context.Context) ([]int64, error) {
	if r.getAllUserIDsFn == nil {
		return nil, nil
	}
	return r.getAllUserIDsFn(ctx)
}

type userNotifierStub struct {
	errorsByUserID map[int64]error
	calls          []int64
}

func (n *userNotifierStub) NotifyUser(tgID int64, _ string) error {
	n.calls = append(n.calls, tgID)
	if err, ok := n.errorsByUserID[tgID]; ok {
		return err
	}
	return nil
}

func TestUserServiceBroadcast(t *testing.T) {
	t.Parallel()

	type want struct {
		sent   int
		failed int
		calls  int
	}

	tests := []struct {
		name           string
		ids            []int64
		repoErr        error
		errorsByUserID map[int64]error
		want           want
	}{
		{
			name: "all messages sent",
			ids:  []int64{1, 2, 3},
			want: want{sent: 3, failed: 0, calls: 3},
		},
		{
			name:           "one user fails",
			ids:            []int64{10, 20, 30},
			errorsByUserID: map[int64]error{20: errors.New("blocked by user")},
			want:           want{sent: 2, failed: 1, calls: 3},
		},
		{
			name:    "repository returns error",
			repoErr: errors.New("db unavailable"),
			want:    want{sent: 0, failed: 0, calls: 0},
		},
		{
			name: "no users to broadcast",
			ids:  []int64{},
			want: want{sent: 0, failed: 0, calls: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &repoStub{
				getAllUserIDsFn: func(context.Context) ([]int64, error) {
					return tt.ids, tt.repoErr
				},
			}
			notifier := &userNotifierStub{errorsByUserID: tt.errorsByUserID}

			svc := &userService{
				repo:         repo,
				userNotifier: notifier,
				logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			sent, failed := svc.Broadcast(context.Background(), "test broadcast")

			if sent != tt.want.sent {
				t.Fatalf("sent mismatch: got %d, want %d", sent, tt.want.sent)
			}
			if failed != tt.want.failed {
				t.Fatalf("failed mismatch: got %d, want %d", failed, tt.want.failed)
			}
			if len(notifier.calls) != tt.want.calls {
				t.Fatalf("notify calls mismatch: got %d, want %d", len(notifier.calls), tt.want.calls)
			}
		})
	}
}
