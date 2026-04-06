package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/mipecx/survey-bot-go/internal/models"
	"github.com/mipecx/survey-bot-go/internal/repository"
)

type UserService interface {
	ProcessMessage(ctx context.Context, tgID int64, username string, text string) (*UserResponse, error)
	ProcessCallback(ctx context.Context, tgID int64, data string) (*UserResponse, error)
}

type userService struct {
	repo   repository.UserRepository
	logger *slog.Logger
}

type UserResponse struct {
	Text    string
	Buttons []string
}

func (s *userService) ProcessCallback(ctx context.Context, tgID int64, data string) (*UserResponse, error) {
	user, err := s.repo.GetOrCreateUser(ctx, tgID, "")
	if err != nil {
		return nil, err
	}

	if user.CurrentForm != "" && user.CurrentForm != "menu" {
		return s.handleSurveyStep(ctx, tgID, user, data)
	}

	switch data {
	case "Да, заполнить полную форму":
		return s.startForm(ctx, tgID, "dating_full")
	case "Нет, спасибо, достаточно":
		return s.handleEndOfForm(ctx, tgID, "") // Покажет обычное меню
	case BtnEvent:
		return s.startForm(ctx, tgID, "event")
	case BtnPartner:
		return s.startForm(ctx, tgID, "dating_short")
	case BtnGodPartner:
		return s.startForm(ctx, tgID, "portrait")
	case BtnGift:
		return &UserResponse{
			Text: "Гайд «5 признаков зрелых отношений» 📖",
			Buttons: []string{
				BtnEvent,
				BtnPartner,
				BtnGodPartner,
				BtnGift,
				BtnConsult,
			},
		}, nil
	case BtnConsult:
		return s.startForm(ctx, tgID, "consult")
	default:
		return s.handleEndOfForm(ctx, tgID, "")
	}
}

// ProcessMessage is the core engine of the service. It routes incoming messages
// based on the user's current state, handles command triggers (like /start),
// validates inputs, and moves the user through the survey steps.
func (s *userService) ProcessMessage(ctx context.Context, tgID int64, username string, text string) (*UserResponse, error) {
	user, err := s.repo.GetOrCreateUser(ctx, tgID, username)
	if err != nil {
		return nil, err
	}

	if text == "/start" {
		return s.handleStartCommand(ctx, tgID, user)
	}

	if user.CurrentForm != "" && user.CurrentForm != "menu" {
		return s.handleSurveyStep(ctx, tgID, user, text)
	}

	return s.handleEndOfForm(ctx, tgID, "")
}

// startForm initializes survey state for user and returns first question.
func (s *userService) startForm(ctx context.Context, tgID int64, formName string) (*UserResponse, error) {
	questions := AllForms[formName]
	firstQ := questions[0]

	if err := s.repo.UpdateForm(ctx, tgID, formName); err != nil {
		s.logger.Error("failed to update form", "error", err)
		return nil, err
	}
	if err := s.repo.UpdateStep(ctx, tgID, firstQ.ID); err != nil {
		s.logger.Error("failed to update step", "error", err)
		return nil, err
	}

	return &UserResponse{Text: firstQ.Text, Buttons: firstQ.Options}, nil
}

func (s *userService) handleStartCommand(ctx context.Context, tgID int64, user *models.User) (*UserResponse, error) {
	question := AllForms["new_user"]
	firstQ := question[0]

	if err := s.repo.UpdateForm(ctx, tgID, "new_user"); err != nil {
		s.logger.Error("failed to update form", "error", err)
		return nil, err
	}
	if err := s.repo.UpdateStep(ctx, tgID, firstQ.ID); err != nil {
		s.logger.Error("failed to update step", "error", err)
		return nil, err
	}

	text := firstQ.Text
	if user.FullName == nil || *user.FullName == "" {
		text = WelcomeText + firstQ.Text
	}

	return &UserResponse{
		Text:    text,
		Buttons: firstQ.Options,
	}, nil

}

func (s *userService) handleSurveyStep(ctx context.Context, tgID int64, user *models.User, text string) (*UserResponse, error) {
	if err := s.validate(user.CurrentStep, text); err != nil {
		return &UserResponse{Text: err.Error()}, nil
	}
	if err := s.repo.SaveAnswer(ctx, tgID, user.CurrentStep, text); err != nil {
		return nil, err
	}

	nextQ := s.getNextQuestion(user.CurrentForm, user.CurrentStep)
	if nextQ == nil {
		return s.handleEndOfForm(ctx, tgID, user.CurrentForm)
	}

	if err := s.repo.UpdateStep(ctx, tgID, nextQ.ID); err != nil {
		s.logger.Error("failed to update step", "error", err)
		return nil, err
	}
	return &UserResponse{Text: nextQ.Text, Buttons: nextQ.Options}, nil
}

// handleEndOfForm returns a response with final instructions or suggestions after a survey form is finished.
func (s *userService) handleEndOfForm(ctx context.Context, tgID int64, currentForm string) (*UserResponse, error) {
	if err := s.repo.UpdateForm(ctx, tgID, ""); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStep(ctx, tgID, ""); err != nil {
		return nil, err
	}

	switch currentForm {
	case "dating_short":
		return &UserResponse{
			Text:    "Вы прошли краткую анкету! Хотите заполнить полную версию?",
			Buttons: []string{"Да, заполнить полную форму", "Нет, спасибо, достаточно"},
		}, nil
	default:
		return &UserResponse{
			Text: "Выберите направление, которое вам сейчас ближе 🤍",
			Buttons: []string{
				BtnEvent,
				BtnPartner,
				BtnGodPartner,
				BtnGift,
				BtnConsult,
			},
		}, nil
	}
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

// validate checks the user's input against the requirements of the current survey step.
//
// TODO: 1. move regex() to global variables to compile once during startup
// 2. use strings.trimSpace
func (s *userService) validate(stepID, answer string) error {
	if answer == "" {
		return fmt.Errorf("ответ не может быть пустым")
	}

	switch stepID {
	case "reg_birthdate":
		_, err := time.Parse("02.01.2006", answer)
		if err != nil {
			return fmt.Errorf("пожалуйста, введите дату в формате ДД.ММ.ГГГГ (например, 20.05.1990)")
		}

	case "reg_phone":
		re := regexp.MustCompile(`^\+?\d{10,15}$`)
		cleanAnswer := regexp.MustCompile(`[\s\-\(\)]`).ReplaceAllString(answer, "")

		if !re.MatchString(cleanAnswer) {
			return fmt.Errorf("пожалуйста, введите корректный номер телефона")
		}
	}

	return nil
}

// NewUserService creates and returns a new instance of the UserService implementation.
func NewUserService(repo repository.UserRepository, logger *slog.Logger) UserService {
	return &userService{
		repo:   repo,
		logger: logger,
	}
}
