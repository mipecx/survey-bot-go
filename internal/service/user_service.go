package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/mipecx/survey-bot-go/internal/repository"
)

type UserService interface {
	ProcessMessage(ctx context.Context, tgID int64, username string, text string) (*UserResponse, error)
}

type userService struct {
	repo   repository.UserRepository
	logger *slog.Logger
}

type UserResponse struct {
	Text    string
	Buttons []string
}

func (s *userService) ProcessMessage(ctx context.Context, tgID int64, username string, text string) (*UserResponse, error) {
	user, err := s.repo.GetOrCreateUser(ctx, tgID, username)
	if err != nil {
		return nil, err
	}

	if text == "/start" {
		firstQ := AllForms["new_user"][0]

		s.repo.UpdateForm(ctx, tgID, "new_user")
		s.repo.UpdateStep(ctx, tgID, firstQ.ID)

		if user.FullName == nil || *user.FullName == "" {
			welcomeText := `Добро пожаловать в пространство Натальи Харисовой 🤍

			Здесь вы можете:
			◦ получить приглашение на закрытые мероприятия знакомств;
			◦ пройти анкету для подбора партнёра;
			◦ записаться на разбор и составление портрета божественного партнёра;
			◦ получить подарочный гайд о зрелых отношениях;
			◦ оставить заявку на личную консультацию.

			Это пространство создано для людей, которым близки зрелые отношения, достойное окружение и красивый уровень общения.

			Чтобы открыть доступ, пройдите короткое знакомство

			` + firstQ.Text

			return &UserResponse{Text: welcomeText, Buttons: firstQ.Options}, nil
		}

		return &UserResponse{Text: firstQ.Text, Buttons: firstQ.Options}, nil
	}

	if text == "Да, заполнить полную форму" && user.CurrentForm == "dating_short" {
		return s.startForm(ctx, tgID, "dating_full")
	}

	if err := s.validate(user.CurrentStep, text); err != nil {
		return &UserResponse{Text: err.Error()}, nil
	}

	if err := s.repo.SaveAnswer(ctx, tgID, user.CurrentStep, text); err != nil {
		return nil, err
	}

	nextQ := s.getNextQuestion(user.CurrentForm, user.CurrentStep)

	if nextQ == nil {
		return s.handleEndOfForm(user.CurrentForm)
	}

	s.repo.UpdateStep(ctx, tgID, nextQ.ID)
	return &UserResponse{
		Text:    nextQ.Text,
		Buttons: nextQ.Options,
	}, nil
}

func (s *userService) startForm(ctx context.Context, tgID int64, formName string) (*UserResponse, error) {
	questions := AllForms[formName]
	firstQ := questions[0]

	s.repo.UpdateForm(ctx, tgID, formName)
	s.repo.UpdateStep(ctx, tgID, firstQ.ID)

	return &UserResponse{Text: firstQ.Text, Buttons: firstQ.Options}, nil
}

func (s *userService) handleEndOfForm(currentForm string) (*UserResponse, error) {
	if currentForm == "dating_short" {
		return &UserResponse{
			Text:    "Вы прошли краткую анкету! Хотите заполнить полную версию?",
			Buttons: []string{"Да, заполнить полную форму", "Нет, спасибо, достаточно"},
		}, nil
	}
	return &UserResponse{Text: "Спасибо! Анкета завершена."}, nil
}

// getNextQuestion returns the next survey question based on the current step.
// If currentStep is empty, the first survey question is returned.
// If the end of the survey has been reached, nil is returned.
//
// NOTE: All IDs within a single form must be unique.
// Duplicate IDs will cause the survey logic to loop.
func (s *userService) getNextQuestion(currentForm, currentStep string) *Question {
	questions, ok := AllForms[currentForm]
	if !ok {
		s.logger.Error("Анкета не найдена")
		return nil
	}
	if currentStep == "" {
		return &questions[0]
	}
	for i, q := range questions {
		if q.ID == currentStep {
			if i+1 < len(questions) {
				return &questions[i+1]
			}
		}
	}
	return nil
}

func (s *userService) validate(stepID, answer string) error {
	if answer == "" {
		return fmt.Errorf("Ответ не может быть пустым")
	}

	switch stepID {
	case "reg_birthdate":
		_, err := time.Parse("02.01.2006", answer)
		if err != nil {
			return fmt.Errorf("Пожалуйста, введите дату в формате ДД.ММ.ГГГГ (например, 20.05.1990)")
		}

	case "reg_phone":
		re := regexp.MustCompile(`^\+?\d{10,15}$`)
		cleanAnswer := regexp.MustCompile(`[\s\-\(\)]`).ReplaceAllString(answer, "")

		if !re.MatchString(cleanAnswer) {
			return fmt.Errorf("Пожалуйста, введите корректный номер телефона")
		}
	}

	return nil
}

func NewUserService(repo repository.UserRepository, logger *slog.Logger) UserService {
	return &userService{
		repo:   repo,
		logger: logger,
	}
}
