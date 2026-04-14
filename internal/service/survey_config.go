package service

type InputType int

const (
	InputText InputType = iota
	InputChoice
	InputPhone
)

// Question represents a single step in a survey form.
// Options, if non-empty, are presented to the user as inline keyboard buttons.
// NextForm is reserved for future branching logic between forms.
type Question struct {
	ID       string
	Text     string
	Options  []string
	NextForm string
	Type     InputType
}

// Main menu button labels used accross all keyboard responses.
const (
	BtnEvent      = "Мероприятия"
	BtnPartner    = "Подбор партнера"
	BtnGodPartner = "Подбор божественного партнера"
	BtnGift       = "Подарок"
	BtnConsult    = "Консультация с Натальей"
)

const WelcomeText = `Добро пожаловать в пространство Натальи Харисовой 🤍

			Здесь вы можете:
			◦ получить приглашение на закрытые мероприятия знакомств;
			◦ пройти анкету для подбора партнёра;
			◦ записаться на разбор и составление портрета божественного партнёра;
			◦ получить подарочный гайд о зрелых отношениях;
			◦ оставить заявку на личную консультацию.

			Это пространство создано для людей, которым близки зрелые отношения, достойное окружение и красивый уровень общения.

			Чтобы открыть доступ, пройдите короткое знакомство

			`

// AllForms is the registry of all available survey forms, keyed by form name.
// Used by the service layer to look up questins for a given form.
var AllForms = map[string][]Question{
	"new_user":     NewUserForm,
	"event":        EventForm,
	"dating_short": DatingShortForm,
	"dating_full":  DatingFullForm,
	"portrait":     PortraitForm,
	"consult":      ConsultForm,
}

var FormEndings = map[string]string{
	"new_user":     "текст для завершения new_user",
	"event":        "текст для завершения event",
	"dating_short": "текст для завершения dating_short",
	"dating_full":  "текст для завершения dating_full",
	"portrait":     "текст для завершения portrait",
	"consult":      "текст для завершения consult",
}

// NewUserForm is the initial registration form shown to every new user.
var NewUserForm = []Question{
	{ID: "reg_name", Text: "Как к вам можно обращаться?", Type: InputText},
	{ID: "reg_phone", Text: "Оставьте ваш контактный номер", Type: InputPhone},
	{ID: "reg_birthdate", Text: "Укажите вашу дату рождения (ДД.ММ.ГГГГ)", Type: InputText},
}

// EventForm collects information for users interested in matchmaking events.
var EventForm = []Question{
	{ID: "event_city", Text: "Ваш город?", Type: InputText},
	{ID: "event_gender", Text: "Ваш пол?", Options: []string{"Женщина", "Мужчина"}, Type: InputChoice},
	{ID: "event_age", Text: "Ваш возраст?", Type: InputText},
	{ID: "event_family_status", Text: "Ваш семейный статус?", Options: []string{"Не был(а) в браке", "В разводе", "Вдовец/Вдова", "В процессе расставания"}, Type: InputChoice},
	{ID: "event_is_repeat", Text: "Были ли вы раньше на наших мероприятиях?", Options: []string{"Да", "Нет"}, Type: InputChoice},
	{ID: "event_goal", Text: "Что вам сейчас ближе?", Options: []string{"Серьёзные отношения", "Знакомство", "Окружение", "Узнать формат"}, Type: InputChoice},
}

// DatingShortForm is the short partner-matching questionnaire.
// On completion, the user is offered to continue with DatingFullForm.
var DatingShortForm = []Question{
	{ID: "dating_short_age", Text: "Ваш возраст?", Type: InputText},
	{ID: "dating_short_city", Text: "Ваш город?", Type: InputText},
	{ID: "dating_short_gender", Text: "Ваш пол?", Options: []string{"Женщина", "Мужчина"}, Type: InputChoice},
	{ID: "dating_short_family_status", Text: "Ваш семейный статус?", Options: []string{"Не был(а) в браке", "В разводе", "Вдовец/Вдова", "В отношениях"}, Type: InputChoice},
	{ID: "dating_short_has_children", Text: "Есть ли у вас дети?", Options: []string{"Да", "Нет"}, Type: InputChoice},
	{ID: "dating_short_format_goal", Text: "Какой формат отношений для вас ближе?", Options: []string{"Брак", "Отношения", "Знакомство", "Присматриваюсь"}, Type: InputChoice},
	{ID: "dating_short_occupation", Text: "Чем вы занимаетесь?", Type: InputText},
	{ID: "dating_short_lifestyle", Text: "Какой у вас образ жизни?", Options: []string{"Активный", "Спокойный", "Смешанный", "В поездках"}, Type: InputChoice},
	{ID: "dating_short_important_traits", Text: "Что для вас важно в партнёре?", Type: InputText},
	{ID: "dating_short_why_now", Text: "Почему именно сейчас вы открыты к отношениям?", NextForm: "DatingFullForm", Type: InputText},
}

// DatingFullForm is the extended partner-matching questionnaire.
var DatingFullForm = []Question{
	{ID: "dating_full_qualities", Text: "Назовите 5 важных качеств вашего партнёра", Type: InputText},
	{ID: "dating_full_unacceptable", Text: "Какие форматы отношений для вас неприемлемы?", Type: InputText},
	{ID: "dating_full_mature_union", Text: "Что для вас значит зрелый союз?", Type: InputText},
	{ID: "dating_full_self_in_relation", Text: "Как вы проявляетесь в отношениях, когда вам хорошо?", Type: InputText},
	{ID: "dating_full_give", Text: "Что вы готовы вкладывать в союз?", Type: InputText},
	{ID: "dating_full_receive", Text: "Что для вас важно получать от партнёра?", Type: InputText},
	{ID: "dating_full_past_patterns", Text: "Какие сценарии из прошлого вы не хотите повторять?", Type: InputText},
	{ID: "dating_full_readiness", Text: "Насколько вы сейчас открыты к реальным встречам?", Type: InputText},
	{ID: "dating_full_values", Text: "Какие ценности партнёра для вас обязательны?", Type: InputText},
	{ID: "dating_full_vision_3y", Text: "Опишите ваш идеальный союз через 3 года", Type: InputText},
	{ID: "dating_full_age_range", Text: "Какие возрастные рамки партнёра для вас комфортны?", Type: InputText},
	{ID: "dating_full_location", Text: "Какие локации проживания для вас подходят?", Type: InputText},
	{ID: "dating_full_status_importance", Text: "Насколько для вас важны статус и доход партнёра?", Type: InputText},
	{ID: "dating_full_past_blockers", Text: "Что чаще всего мешало отношениям складываться раньше?", Type: InputText},
	{ID: "dating_full_recognition", Text: "Как вы понимаете, что человек «ваш»?", Type: InputText},
}

// PortraitForm collects user's focus area for divine partner protrait session.
var PortraitForm = []Question{
	{ID: "portrait_main_focus", Text: "Что для вас сейчас наиболее важно?", Options: []string{"Понять, кто мне подходит", "Почему не складываются отношения", "Повторяющийся сценарий", "Создать зрелый союз"}, Type: InputChoice},
}

// ConsultForm collects a free-form request for a personal consultation.
var ConsultForm = []Question{
	{ID: "consult_request", Text: "Опишите ваш запрос в свободной форме", Type: InputText},
}
