package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mipecx/survey-bot-go/internal/models"
	"github.com/mipecx/survey-bot-go/internal/repository"
)

type AdminNotifier interface {
	Notify(text string) error
}

// UserService defines the contract for processing incoming Telegram updates.
type UserService interface {
	ProcessMessage(ctx context.Context, tgID int64, username string, text string) (*UserResponse, error)
	ProcessCallback(ctx context.Context, tgID int64, username string, data string) (*UserResponse, error)
}

type userService struct {
	repo     repository.UserRepository
	logger   *slog.Logger
	notifier AdminNotifier
	admins   map[int64]bool
}

// UserResponse holds the bot's reply text and optional inline keyboard buttons.
type UserResponse struct {
	Text      string
	Buttons   []string
	InputType InputType
}

var (
	phoneRe  = regexp.MustCompile(`^\+?\d{10,15}$`)
	spacesRe = regexp.MustCompile(`[\s\-\(\)]`)
)

const FormMainMenu = "main_menu"

// ProcessCallback handles inline keyboard button presses.
// If the user is mid-survey, the callback data is treated as an answer.
func (s *userService) ProcessCallback(ctx context.Context, tgID int64, username string, data string) (*UserResponse, error) {
	user, err := s.repo.GetOrCreateUser(ctx, tgID, username)
	if err != nil {
		s.logger.Error("Failed to get or update the user", "user_id", tgID, "error", err)
		return nil, err
	}

	if user.CurrentForm != FormMainMenu && user.CurrentForm != "" {
		return s.handleSurveyStep(ctx, tgID, user, data, true)
	}

	switch data {
	case "Да, заполнить полную форму":
		return s.startForm(ctx, tgID, "dating_full")
	case "Нет, спасибо, достаточно":
		return s.handleEndOfForm(ctx, tgID, FormMainMenu)
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
		return s.handleEndOfForm(ctx, tgID, FormMainMenu)
	}
}

// ProcessMessage is the core engine of the service. It routes incoming messages
// based on the user's current state, handles command triggers (like /start),
// validates inputs, and moves the user through the survey steps.
func (s *userService) ProcessMessage(ctx context.Context, tgID int64, username string, text string) (*UserResponse, error) {
	user, err := s.repo.GetOrCreateUser(ctx, tgID, username)
	if err != nil {
		s.logger.Error("Failed to get or update the user",
			"user_id", tgID,
			"error", err)
		return nil, err
	}

	if text == "/start" {
		return s.handleStartCommand(ctx, tgID, user)
	}

	if user.CurrentForm != FormMainMenu && user.CurrentForm != "" {
		return s.handleSurveyStep(ctx, tgID, user, text, false)
	}

	return s.handleEndOfForm(ctx, tgID, FormMainMenu)
}

// startForm initializes survey state for user and returns first question.
func (s *userService) startForm(ctx context.Context, tgID int64, formName string) (*UserResponse, error) {
	questions := AllForms[formName]
	if len(questions) == 0 {
		return nil, fmt.Errorf("form %s not found", formName)
	}
	firstQ := questions[0]

	if err := s.repo.UpdateForm(ctx, tgID, formName); err != nil {
		s.logger.Error("failed to update user form",
			"user_id", tgID,
			"form_id", formName,
			"error", err)
		return nil, err
	}

	if err := s.repo.UpdateStep(ctx, tgID, firstQ.ID); err != nil {
		s.logger.Error("failed to update step",
			"user_id", tgID,
			"step_id", formName,
			"error", err)
		return nil, err
	}

	return &UserResponse{Text: firstQ.Text, Buttons: firstQ.Options}, nil
}

// handleStartCommand resets user into the new_user registration form.
// Prepends WeclomeText for users who have not yet provided their name.
func (s *userService) handleStartCommand(ctx context.Context, tgID int64, user *models.User) (*UserResponse, error) {
	s.logger.Info("start command received", "user_id", tgID)
	questions := AllForms["new_user"]
	if len(questions) == 0 {
		return nil, fmt.Errorf("form %s not found", "new_user")
	}
	firstQ := questions[0]

	if err := s.repo.UpdateForm(ctx, tgID, "new_user"); err != nil {
		s.logger.Error("failed to update form",
			"user_id", tgID,
			"form_id", "new_user",
			"error", err)
		return nil, err
	}
	if err := s.repo.UpdateStep(ctx, tgID, firstQ.ID); err != nil {
		s.logger.Error("failed to update step",
			"user_id", tgID,
			"step_id", firstQ.ID,
			"error", err)
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

// handleSurveyStep validates and saves the user's answer for the current step,
// then advances to the next question or ends the form.
func (s *userService) handleSurveyStep(ctx context.Context, tgID int64, user *models.User, text string, isCallback bool) (*UserResponse, error) {
	text = strings.TrimSpace(text)

	questions := AllForms[user.CurrentForm]
	var currentQ *Question
	for i := range questions {
		if questions[i].ID == user.CurrentStep {
			currentQ = &questions[i]
			break
		}
	}

	if currentQ.Type == InputChoice && !isCallback {
		return &UserResponse{
			Text:    "Пожалуйста, выберите вариант из списка ниже\n\n" + currentQ.Text,
			Buttons: currentQ.Options,
		}, nil
	}

	if err := s.validate(currentQ, text); err != nil {
		return &UserResponse{Text: err.Error(), Buttons: currentQ.Options}, nil
	}

	if err := s.repo.SaveAnswer(ctx, tgID, user.CurrentStep, text); err != nil {
		s.logger.Error("failed to save answer",
			"user_id", tgID,
			"step_id", user.CurrentStep,
			"text", text,
			"error", err)
		return nil, err
	}

	nextQ := s.getNextQuestion(user.CurrentForm, user.CurrentStep)

	if nextQ == nil {
		return s.handleEndOfForm(ctx, tgID, user.CurrentForm)
	}

	if err := s.repo.UpdateStep(ctx, tgID, nextQ.ID); err != nil {
		s.logger.Error("failed to update step",
			"user_id", tgID,
			"step_id", nextQ.ID,
			"error", err)
		return nil, err
	}
	return &UserResponse{Text: nextQ.Text, Buttons: nextQ.Options, InputType: nextQ.Type}, nil
}

// handleEndOfForm returns a response with final instructions or suggestions after a survey form is finished.
func (s *userService) handleEndOfForm(ctx context.Context, tgID int64, currentForm string) (*UserResponse, error) {
	if err := s.repo.UpdateForm(ctx, tgID, FormMainMenu); err != nil {
		s.logger.Error("failed to update form",
			"user_id", tgID,
			"form_id", currentForm,
			"error", err)
		return nil, err
	}
	if err := s.repo.UpdateStep(ctx, tgID, ""); err != nil {
		s.logger.Error("failed to update step",
			"user_id", tgID,
			"form_id", currentForm,
			"step_id", "",
			"error", err)
		return nil, err
	}

	if currentForm != FormMainMenu && currentForm != "" {
		go s.notifyAdmin(tgID, currentForm)
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
		s.logger.Error("Анкета не найдена",
			"form_id", currentForm,
			"step_id", currentStep)
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
func (s *userService) validate(q *Question, answer string) error {
	if answer == "" {
		return fmt.Errorf("ответ не может быть пустым")
	}

	switch q.Type {
	case InputPhone:
		if !phoneRe.MatchString(spacesRe.ReplaceAllString(answer, "")) {
			return fmt.Errorf("пожалуйста, введите корректный номер телефона")
		}
	}

	switch q.ID {
	case "reg_birthdate":
		_, err := time.Parse("02.01.2006", answer)
		if err != nil {
			return fmt.Errorf("пожалуйста, введите дату в формате ДД.ММ.ГГГГ (например, 20.05.1990)")
		}

	case "reg_phone":
		if !phoneRe.MatchString(spacesRe.ReplaceAllString(answer, "")) {
			return fmt.Errorf("пожалуйста, введите корректный номер телефона")
		}
	case "event_age", "dating_short_age":
		age, err := strconv.Atoi(answer)
		if err != nil {
			return fmt.Errorf("возраст должен быть числом")
		}
		if age < 18 || age > 99 {
			return fmt.Errorf("возраст должен быть в пределах от 18 до 99")
		}
	}

	return nil
}

// NewUserService creates and returns a new instance of the UserService implementation.
func NewUserService(repo repository.UserRepository, logger *slog.Logger, notifier AdminNotifier, adminMap map[int64]bool) UserService {
	return &userService{
		repo:     repo,
		logger:   logger,
		notifier: notifier,
		admins:   adminMap,
	}
}

func (s *userService) notifyAdmin(tgID int64, formID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := s.repo.GetOrCreateUser(ctx, tgID, "")
	if err != nil {
		s.logger.Error("notifyAdmin: user fetch failed", "err", err)
		return
	}

	answers, err := s.repo.GetAnswersByForm(ctx, tgID)
	if err != nil {
		s.logger.Error("notifyAdmin: answers fetch failed", "err", err)
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Новая анкета: %s</b>\n", formID))
	sb.WriteString(fmt.Sprintf("<b>Клиент:</b> %s (@%s)\n", *user.FullName, user.Username))
	sb.WriteString(fmt.Sprintf("<b>ID:</b> <code>%d</code>\n", tgID))
	sb.WriteString("---------------------------\n\n")

	if questions, ok := AllForms[formID]; ok {
		for _, q := range questions {
			if val, exists := answers[q.ID]; exists {
				sb.WriteString(fmt.Sprintf("<b>%s</b>\n%s\n\n", q.Text, val))
			}
		}
	}

	if err := s.notifier.Notify(sb.String()); err != nil {
		s.logger.Error("notifyAdmin: send failed", "err", err)
	}
}
