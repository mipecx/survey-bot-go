package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mipecx/survey-bot-go/internal/config"
	"github.com/mipecx/survey-bot-go/internal/ctxlog"
	"github.com/mipecx/survey-bot-go/internal/models"
	"github.com/mipecx/survey-bot-go/internal/repository"
)

var (
	ErrFormNotFound = errors.New("Простите, этот раздел отсутствует, решим вопрос в ближайшее время")
	ErrInternal     = errors.New("Простите, произошла ошибка сервера, решим вопрос в ближайшее время")
	ErrEmptyAnswer  = errors.New("Ответ не может быть пустым")
	ErrInvalidPhone = errors.New("Пожалуйста, введите корректный номер телефона")
	ErrInvalidDate  = errors.New("Пожалуйста, введите дату в формате ДД.ММ.ГГГГ")
	ErrInvalidAge   = errors.New("Возраст должен быть в пределах от 18 до 99")
	ErrNotANumber   = errors.New("Пожалуйста, введите число")
)

const FormNewUser = "new_user"

type AdminNotifier interface {
	Notify(text string) error
}

// UserService defines the contract for processing incoming Telegram updates.
type UserService interface {
	ProcessMessage(ctx context.Context, tgID int64, username string, text string) (*UserResponse, error)
	ProcessCallback(ctx context.Context, tgID int64, username string, data string) (*UserResponse, error)
}

func GetGiftID() string {
	url := os.Getenv("GIFT_FILE_ID")
	if url == "" {
		return ""
	}
	return url
}

type userService struct {
	repo        repository.UserRepository
	logger      *slog.Logger
	notifier    AdminNotifier
	admins      map[int64]bool
	giftFileID  string
	ImageFileID string
}

// UserResponse holds the bot's reply text and optional inline keyboard buttons.
type UserResponse struct {
	Text             string
	Buttons          []string
	InputType        InputType
	StepID           string
	Document         string
	MessageID        int
	Edit             bool
	SendWelcomeImage bool
}

var (
	phoneRe  = regexp.MustCompile(`^\+?\d{10,15}$`)
	spacesRe = regexp.MustCompile(`[\s\-\(\)]`)
)

const FormMainMenu = "main_menu"

// ProcessCallback handles inline keyboard button presses.
// If the user is mid-survey, the callback data is treated as an answer.
func (s *userService) ProcessCallback(ctx context.Context, tgID int64, username string, data string) (*UserResponse, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	var callbackStepID, cleanValue string

	if parts := strings.SplitN(data, ":", 2); len(parts) == 2 {
		callbackStepID = parts[0]
		cleanValue = parts[1]
	} else {
		cleanValue = data
	}

	user, err := s.repo.GetOrCreateUser(ctx, tgID, username)
	if err != nil {
		logger.Error("Failed to get or update the user", "user_id", tgID, "error", err)
		return nil, err
	}

	if callbackStepID != "" && user.CurrentStep != callbackStepID {
		logger.Warn("Old callback ignored", "expected", user.CurrentStep, "got", callbackStepID)
		return nil, nil
	}

	if user.CurrentForm != FormMainMenu && user.CurrentForm != "" {
		return s.handleSurveyStep(ctx, tgID, user, cleanValue, true)
	}

	switch data {
	case "Да, заполнить полную форму":
		return s.startForm(ctx, tgID, "dating_full", true)
	case "Нет, спасибо, достаточно":
		return s.handleEndOfForm(ctx, tgID, FormMainMenu)
	case BtnEvent:
		return s.startFormOrCollectContact(ctx, tgID, user, "event")
	case BtnPartner:
		return s.startFormOrCollectContact(ctx, tgID, user, "dating_short")
	case BtnGodPartner:
		return s.startFormOrCollectContact(ctx, tgID, user, "portrait")
	case BtnGift:
		return &UserResponse{
			Text: "Гайд «5 признаков зрелых отношений» 📖",
			Buttons: []string{
				BtnToMainMenu,
			},
			Document: s.giftFileID,
		}, nil
	case BtnConsult:
		return s.startFormOrCollectContact(ctx, tgID, user, "consult")
	case BtnToMainMenu:
		return &UserResponse{
			Text:    "Выберите направление, которое вам сейчас ближе 🤍",
			Buttons: MainMenuButtons,
		}, nil
	default:
		return s.handleEndOfForm(ctx, tgID, FormMainMenu)
	}
}

// ProcessMessage is the core engine of the service. It routes incoming messages
// based on the user's current state, handles command triggers (like /start),
// validates inputs, and moves the user through the survey steps.
func (s *userService) ProcessMessage(ctx context.Context, tgID int64, username string, text string) (*UserResponse, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	user, err := s.repo.GetOrCreateUser(ctx, tgID, username)
	if err != nil {
		logger.Error("Failed to get or update the user",
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
func (s *userService) startForm(ctx context.Context, tgID int64, formName string, edit bool) (*UserResponse, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	questions, ok := AllForms[formName]
	if !ok || len(questions) == 0 {
		logger.Error("Form configuration missing", "form_name", formName, "user_id", tgID)
		return nil, ErrFormNotFound
	}
	firstQ := questions[0]

	if err := s.repo.UpdateForm(ctx, tgID, formName); err != nil {
		logger.Error("Failed to update user form",
			"user_id", tgID,
			"form_id", formName,
			"error", err)
		return nil, err
	}

	if err := s.repo.UpdateStep(ctx, tgID, firstQ.ID); err != nil {
		logger.Error("Failed to update step",
			"user_id", tgID,
			"step_id", formName,
			"error", err)
		return nil, err
	}

	return &UserResponse{
		Text:      firstQ.Text,
		Buttons:   firstQ.Options,
		Edit:      edit,
		InputType: firstQ.Type,
		StepID:    firstQ.ID,
	}, nil
}

// handleStartCommand resets user into the new_user registration form.
// Prepends WeclomeText for users who have not yet provided their name.
func (s *userService) handleStartCommand(ctx context.Context, tgID int64, user *models.User) (*UserResponse, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	logger.Info("Start command received", "user_id", tgID)

	isRegistered := user.FullName != nil && *user.FullName != ""

	if !isRegistered {
		questions, ok := AllForms[FormNewUser]
		if !ok || len(questions) == 0 {
			logger.Error("Form configuration missing", "form_id", FormNewUser)
			return nil, ErrFormNotFound
		}
		firstQ := questions[0]

		if err := s.repo.UpdateForm(ctx, tgID, "new_user"); err != nil {
			logger.Error("Failed to update form",
				"user_id", tgID,
				"form_id", "new_user",
				"error", err)
			return nil, err
		}
		if err := s.repo.UpdateStep(ctx, tgID, firstQ.ID); err != nil {
			logger.Error("Failed to update step",
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
			Text:             text,
			Buttons:          firstQ.Options,
			SendWelcomeImage: true,
		}, nil
	}
	if err := s.repo.ResetUserProgress(ctx, tgID, ""); err != nil {
		return nil, err
	}

	return &UserResponse{
		Text:    "Рады видеть вас снова! Что вас интересует сегодня?",
		Buttons: MainMenuButtons,
		StepID:  "",
	}, nil
}

// handleSurveyStep validates and saves the user's answer for the current step,
// then advances to the next question or ends the form.
func (s *userService) handleSurveyStep(ctx context.Context, tgID int64, user *models.User, text string, isCallback bool) (*UserResponse, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	text = strings.TrimSpace(text)

	questions, ok := AllForms[user.CurrentForm]
	if !ok || len(questions) == 0 {
		logger.Error("Form not found or empty", "form_id", user.CurrentForm, "user_id", tgID)
		return s.handleStartCommand(ctx, tgID, user)
	}

	var currentQ *Question
	for i := range questions {
		if questions[i].ID == user.CurrentStep {
			currentQ = &questions[i]
			break
		}
	}

	if currentQ == nil {
		logger.Warn("Current step not found, resetting to start",
			"user_id", tgID,
			"current_step", user.CurrentStep)
		return s.handleStartCommand(ctx, tgID, user)
	}

	if currentQ.Type == InputChoice && !isCallback {
		return &UserResponse{
			Text:    "Пожалуйста, выберите вариант, нажав на кнопку ниже:\n\n" + currentQ.Text,
			Buttons: currentQ.Options,
			Edit:    true,
		}, nil
	}

	if err := s.validate(currentQ, text); err != nil {
		return &UserResponse{
			Text:      err.Error(),
			Buttons:   currentQ.Options,
			Edit:      true,
			InputType: currentQ.Type,
		}, nil
	}

	if err := s.repo.SaveAnswer(ctx, tgID, user.CurrentStep, text); err != nil {
		return nil, err
	}

	nextQ := s.getNextQuestion(ctx, user.CurrentForm, user.CurrentStep)

	if nextQ == nil {
		logger.Info("User finished form", "user_id", tgID, "form_id", user.CurrentForm)
		return s.handleEndOfForm(ctx, tgID, user.CurrentForm)
	}

	if err := s.repo.UpdateStep(ctx, tgID, nextQ.ID); err != nil {
		return nil, err
	}

	return &UserResponse{
		Text:      nextQ.Text,
		Buttons:   nextQ.Options,
		InputType: nextQ.Type,
		StepID:    nextQ.ID,
	}, nil
}

// handleEndOfForm returns a response with final instructions or suggestions after a survey form is finished.
func (s *userService) handleEndOfForm(ctx context.Context, tgID int64, currentForm string) (*UserResponse, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	if err := s.repo.ResetUserProgress(ctx, tgID, FormMainMenu); err != nil {
		logger.Error("Failed to update form",
			"user_id", tgID,
			"form_id", currentForm,
			"error", err)
		return nil, err
	}

	if currentForm != FormMainMenu && currentForm != "" {
		go s.notifyAdmin(ctx, tgID, currentForm)
	}

	message, ok := FormEndings[currentForm]
	if !ok {
		message = "Вернитесь в главное меню"
	}

	var buttons []string
	switch currentForm {
	case "dating_short":
		message = "Вы прошли краткую анкету! Хотите заполнить полную версию?"
		buttons = []string{"Да, заполнить полную форму", BtnToMainMenu}
	case "new_user":
		buttons = MainMenuButtons
	case "contact":
		user, err := s.repo.GetOrCreateUser(ctx, tgID, "")
		if err != nil {
			return nil, err
		}
		pending := user.PendingForm
		if err := s.repo.ClearPendingForm(ctx, tgID); err != nil {
			return nil, err
		}
		if pending != nil && *pending != "" {
			return s.startForm(ctx, tgID, *pending, true)
		}
		buttons = MainMenuButtons
		message = FormEndings["new_user"]
	default:
		buttons = []string{
			BtnToMainMenu,
		}
	}

	return &UserResponse{
		Text:    message,
		Buttons: buttons,
		Edit:    currentForm != "new_user",
	}, nil
}

// getNextQuestion returns the next survey question based on the current step.
// If currentStep is empty, the first survey question is returned.
// If the end of the survey has been reached, nil is returned.
//
// NOTE: All IDs within a single form must be unique.
// Duplicate IDs will cause the survey logic to loop.
func (s *userService) getNextQuestion(ctx context.Context, currentForm, currentStep string) *Question {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	questions, ok := AllForms[currentForm]
	if !ok {
		logger.Error("Form not found",
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
		return ErrEmptyAnswer
	}

	switch q.Type {
	case InputPhone:
		if !phoneRe.MatchString(spacesRe.ReplaceAllString(answer, "")) {
			return ErrInvalidPhone
		}
	}

	switch q.ID {
	case "reg_birthdate":
		if _, err := time.Parse("02.01.2006", answer); err != nil {
			return ErrInvalidDate
		}

	case "event_age", "dating_short_age":
		age, err := strconv.Atoi(answer)
		if err != nil {
			return ErrNotANumber
		}
		if age < 18 || age > 99 {
			return ErrInvalidAge
		}
	}

	return nil
}

// NewUserService creates and returns a new instance of the UserService implementation.
func NewUserService(repo repository.UserRepository, logger *slog.Logger, notifier AdminNotifier, cfg *config.Config) UserService {
	return &userService{
		repo:       repo,
		logger:     logger,
		notifier:   notifier,
		admins:     cfg.AdminIDs,
		giftFileID: cfg.GiftFileID,
	}
}

func (s *userService) notifyAdmin(ctx context.Context, tgID int64, formID string) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user, err := s.repo.GetOrCreateUser(ctx, tgID, "")
	if err != nil {
		logger.Error("NotifyAdmin: user fetch failed", "error", err)
		return
	}

	answers, err := s.repo.GetAnswersByForm(ctx, tgID)
	if err != nil {
		logger.Error("NotifyAdmin: answers fetch failed", "error", err)
		return
	}

	var sb strings.Builder

	fullName := "Не указано"
	if user.FullName != nil && *user.FullName != "" {
		fullName = *user.FullName
	}

	username := "скрыт"
	if user.Username != "" {
		username = "@" + user.Username
	}

	fmt.Fprintf(&sb, "<b>Новая анкета: %s</b>\n", formID)
	fmt.Fprintf(&sb, "<b>Клиент:</b> %s (@%s)\n", fullName, username)
	fmt.Fprintf(&sb, "<b>ID:</b> <code>%d</code>\n", tgID)
	fmt.Fprintf(&sb, "-------------------------\n\n")
	if questions, ok := AllForms[formID]; ok {
		for _, q := range questions {
			if val, exists := answers[q.ID]; exists {
				fmt.Fprintf(&sb, "<b>%s</b>\n%s\n\n", q.Text, val)
			}
		}
	}

	if err := s.notifier.Notify(sb.String()); err != nil {
		logger.Error("NotifyAdmin: send failed", "error", err)
	}
}

func (s *userService) isContactComplete(user *models.User) bool {
	return user.Phone != nil && *user.Phone != "" && user.BirthDate != nil
}

func (s *userService) startFormOrCollectContact(ctx context.Context, tgID int64, user *models.User, targetForm string) (*UserResponse, error) {
	if s.isContactComplete(user) {
		return s.startForm(ctx, tgID, targetForm, true)
	}

	if error := s.repo.SetPendingForm(ctx, tgID, targetForm); error != nil {
		return nil, error
	}
	return s.startForm(ctx, tgID, "contact", true)
}
