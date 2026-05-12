// Package service implements the core business logic of the survey bot.
// It orchestrates user registration, survey flow, answer validation,
// admin notifications, and Google Sheets integration.
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
	"github.com/mipecx/survey-bot-go/internal/sheets"
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

// AdminNotifier is implemented by any type that can deliver
// a formatted notification to administrator.
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

// userService is concrete implementation of UserService.
// It is intentionally unexported - construct it via NewUserService.
type userService struct {
	repo        repository.UserRepository
	logger      *slog.Logger
	notifier    AdminNotifier
	admins      map[int64]bool
	giftFileID  string
	ImageFileID string
	sheets      *sheets.Client
}

// UserResponse hold's bot reply to a single user interaction.
// Edit=true instructs the handler to edit the previous bot messagein place
// rather than sending a new one.
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

	user, err := s.repo.GetOrCreateUser(ctx, tgID, username)
	if err != nil {
		logger.Error("Failed to get or update the user", "error", err)
		return nil, err
	}

	var callbackStepID, cleanValue string

	if parts := strings.SplitN(data, ":", 2); len(parts) == 2 {
		callbackStepID = parts[0]
		raw := parts[1]

		if idx, err := strconv.Atoi(raw); err == nil {
			cleanValue = resolveOption(user.CurrentForm, callbackStepID, idx, raw)
		} else {
			cleanValue = raw
		}
	} else {
		cleanValue = data
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
	/* case BtnGift:
	return &UserResponse{
		Text: "Гайд «5 признаков зрелых отношений» 📖",
		Buttons: []string{
			BtnToMainMenu,
		},
		Document: s.giftFileID,
	}, nil
	*/
	case BtnConsult:
		return s.startFormOrCollectContact(ctx, tgID, user, "consult")
	case BtnWebinar:
		return s.startFormOrCollectContact(ctx, tgID, user, "webinar")
	case BtnProgram:
		return s.startFormOrCollectContact(ctx, tgID, user, "authors_programm")
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
		logger.Error("Form configuration missing", "form_name", formName)
		return nil, ErrFormNotFound
	}
	firstQ := questions[0]

	if err := s.repo.UpdateForm(ctx, tgID, formName); err != nil {
		logger.Error("Failed to update user form",
			"form_id", formName,
			"error", err)
		return nil, err
	}

	if fields, ok := FormAutoFields[formName]; ok {
		user, err := s.repo.GetOrCreateUser(ctx, tgID, "")
		if err == nil {
			for answerKey, profileField := range fields {
				switch profileField {
				case "age":
					if user.BirthDate != nil {
						years := time.Now().Year() - user.BirthDate.Year()
						if time.Now().YearDay() < user.BirthDate.YearDay() {
							years--
						}
						_ = s.repo.SaveAnswer(ctx, tgID, answerKey, fmt.Sprintf("%d", years))
					}
				case "city":
					if user.City != nil && *user.City != "" {
						_ = s.repo.SaveAnswer(ctx, tgID, answerKey, *user.City)
					}
				case "gender":
					if user.Gender != nil && *user.Gender != "" {
						_ = s.repo.SaveAnswer(ctx, tgID, answerKey, *user.Gender)
					}
				}
			}
		}
	}

	if err := s.repo.UpdateStep(ctx, tgID, firstQ.ID); err != nil {
		logger.Error("Failed to update step",
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
// Prepends WelcomeText for users who have not yet provided their name.
func (s *userService) handleStartCommand(ctx context.Context, tgID int64, user *models.User) (*UserResponse, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)
	logger.Info("Start command received")

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
				"form_id", "new_user",
				"error", err)
			return nil, err
		}
		if err := s.repo.UpdateStep(ctx, tgID, firstQ.ID); err != nil {
			logger.Error("Failed to update step",
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
		Text:    "Рады, что вы заглянули в пространство Архитектуры любви. С чего вы хотели бы начать сегодня?",
		Buttons: MainMenuButtons,
		StepID:  "",
	}, nil
}

// handleSurveyStep validates and saves the user's answer for the current step,
// then advances to the next question or ends the form.
func (s *userService) handleSurveyStep(ctx context.Context, tgID int64, user *models.User, text string, isCallback bool) (*UserResponse, error) {
	logger := ctxlog.LoggerFromCtx(ctx, s.logger)

	text = strings.TrimSpace(text)

	if text == BtnToMainMenu {
		if err := s.repo.UpdateForm(ctx, tgID, string(FormMainMenu)); err != nil {
			return nil, err
		}
		return s.handleStartCommand(ctx, tgID, user)
	}

	questions, ok := AllForms[user.CurrentForm]
	if !ok || len(questions) == 0 {
		logger.Error("Form not found or empty", "form_id", user.CurrentForm)
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
		logger.Info("User finished form", "form_id", user.CurrentForm)
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
			return s.startForm(ctx, tgID, *pending, false)
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
		Edit:    currentForm != "new_user" && lastQuestionType(currentForm) != InputText,
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
func NewUserService(repo repository.UserRepository, logger *slog.Logger, notifier AdminNotifier, cfg *config.Config, sheetsClient *sheets.Client) UserService {
	return &userService{
		repo:       repo,
		logger:     logger,
		notifier:   notifier,
		admins:     cfg.AdminIDs,
		giftFileID: cfg.GiftFileID,
		sheets:     sheetsClient,
	}
}

// notifyAdmin sends a formatted HTML summary of a completed survey to all admins
// and appends a row to the corresponding Google Sheet.
// It runs in a separate goroutine and uses its own timeout context.
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

	age := ""
	if user.BirthDate != nil {
		years := time.Now().Year() - user.BirthDate.Year()
		if time.Now().YearDay() < user.BirthDate.YearDay() {
			years--
		}
		age = fmt.Sprintf("%d", years)
	}
	city := ""
	if user.City != nil {
		city = *user.City
	}
	gender := ""
	if user.Gender != nil {
		gender = *user.Gender
	}

	fmt.Fprintf(&sb, "<b>Новая анкета: %s</b>\n", formID)
	fmt.Fprintf(&sb, "<b>Клиент:</b> %s (%s)\n", fullName, username)
	fmt.Fprintf(&sb, "<b>ID:</b> <code>%d</code>\n", tgID)
	fmt.Fprintf(&sb, "<b>Возраст:</b> %s\n", age)
	fmt.Fprintf(&sb, "<b>Город:</b> %s\n", city)
	fmt.Fprintf(&sb, "<b>Пол:</b> %s\n", gender)
	fmt.Fprintf(&sb, "-------------------------\n\n")
	if questions, ok := AllForms[formID]; ok {
		for _, q := range questions {
			if q.InfoOnly {
				continue
			}
			if val, exists := answers[q.ID]; exists {
				fmt.Fprintf(&sb, "<b>%s</b>\n%s\n\n", q.Text, val)
			}
		}
	}

	if err := s.notifier.Notify(sb.String()); err != nil {
		logger.Error("NotifyAdmin: send failed", "error", err)
	}

	if s.sheets != nil {
		sheetName := formSheetName(formID)
		row := buildSheetRow(formID, tgID, fullName, username, age, city, gender, answers)
		if err := s.sheets.AppendRow(ctx, sheetName, row); err != nil {
			logger.Error("NotifyAdmin: sheets append failed", "error", err)
		}
	}
}

// isContactComplete reports whether the user rovided all required
// contact details (phone, birth date, city, gender).
func (s *userService) isContactComplete(user *models.User) bool {
	return user.Phone != nil && *user.Phone != "" && user.BirthDate != nil && user.City != nil && user.Gender != nil
}

// startFormOrCollectContact starts targetForm if the user's contact profile
// is complete. Otherwise it saves targetForm as pending and redirects
// the user to he contact collection flow first.
func (s *userService) startFormOrCollectContact(ctx context.Context, tgID int64, user *models.User, targetForm string) (*UserResponse, error) {
	if s.isContactComplete(user) {
		return s.startForm(ctx, tgID, targetForm, true)
	}

	if error := s.repo.SetPendingForm(ctx, tgID, targetForm); error != nil {
		return nil, error
	}
	return s.startForm(ctx, tgID, "contact", true)
}

// resloveOptions maps a zero-based button index back to the original option text.
// Used to convert compact callback_data (stepID:index) into human readable answer.
// Falls back to the raw index string if the form or index is not found.
func resolveOption(formName, stepID string, idx int, fallback string) string {
	questions, ok := AllForms[formName]
	if !ok {
		return fallback
	}
	for _, q := range questions {
		if q.ID == stepID && idx < len(q.Options) {
			return q.Options[idx]
		}
	}
	return fallback
}

func lastQuestionType(formName string) InputType {
	questions, ok := AllForms[formName]
	if !ok || len(questions) == 0 {
		return InputText
	}
	return questions[len(questions)-1].Type
}

// formSheetName maps a form ID to its display name in Google Sheets
func formSheetName(formID string) string {
	names := map[string]string{
		"new_user":         "Регистрация",
		"contact":          "Контакты",
		"event":            "Вечера",
		"dating_short":     "Подбор партнёра (краткая)",
		"dating_full":      "Подбор партнёра (полная)",
		"portrait":         "Портрет партнёра",
		"consult":          "Консультация",
		"authors_programm": "Программа",
		"webinar":          "Вебинар 14 мая",
	}
	if name, ok := names[formID]; ok {
		return name
	}
	return formID
}

// buildSheetRow assembles a flat ro of values for Google Sheets from
// user profile fields and survey answers in the order defined by AllForms.
func buildSheetRow(formID string, tgID int64, fullName, username, age, city, gender string, answers map[string]string) []any {
	row := []any{
		time.Now().Format("02.01.2006 15:04"),
		fullName,
		username,
		fmt.Sprintf("%d", tgID),
		age,
		city,
		gender,
	}
	if questions, ok := AllForms[formID]; ok {
		for _, q := range questions {
			if q.InfoOnly {
				continue
			}
			row = append(row, answers[q.ID])
		}
	}
	return row
}

// BuildSheetConfigs generates a SheetsConfig entries for all registered forms,
// using base profile headers followed by each non-inforamtional question text.
// Called once at startup to initialise the Google Sheets structure.
func BuildSheetConfigs() []sheets.SheetConfig {
	baseHeaders := []string{"Дата", "Имя", "Username", "ID", "Возраст", "Город", "Пол"}

	var configs []sheets.SheetConfig
	for formID, questions := range AllForms {
		headers := make([]string, len(baseHeaders))
		copy(headers, baseHeaders)
		for _, q := range questions {
			if !q.InfoOnly {
				headers = append(headers, q.Text)
			}
		}
		configs = append(configs, sheets.SheetConfig{
			Name:    formSheetName(formID),
			Headers: headers,
		})
	}
	return configs
}
