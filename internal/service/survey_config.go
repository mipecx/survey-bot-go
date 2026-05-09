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
	InfoOnly bool
}

// Main menu button labels used accross all keyboard responses.
const (
	BtnEvent      = "Закрытые вечера Архитектуры любви"
	BtnPartner    = "Индивидуальный подбор партнёра"
	BtnGodPartner = "Портрет божественного партнёра"
	BtnGift       = "Гайды и чек-листы"
	BtnConsult    = "Консультация с Натальей"
	BtnCommunity  = "Пространство наполненной женщины"
	BtnToMainMenu = "Вернуться в главное меню"
	BtnProgram    = "«Путь к своему мужчине»"
)

// FormAutoFields — fields that automatically pulled from user's profile
var FormAutoFields = map[string]map[string]string{
	"event": {
		"event_age":    "age",
		"event_city":   "city",
		"event_gender": "gender",
	},
	"dating_short": {
		"dating_short_age":    "age",
		"dating_short_city":   "city",
		"dating_short_gender": "gender",
	},
}

var (
	MainMenuButtons = []string{
		BtnProgram,
		BtnConsult,
		BtnGodPartner,
		BtnGift,
		BtnPartner,
		BtnEvent,
		BtnCommunity,
	}
)

const WelcomeText = `Добро пожаловать в Архитектуру любви 🤍

Это закрытое пространство для тех, кому близки зрелые отношения, достойное окружение и красивый уровень общения.

Здесь не про случайные знакомства — здесь про осознанный выбор и создание счастливого союза.

Чтобы подобрать для вас подходящий путь, ответьте на несколько коротких вопросов.

`

// AllForms is the registry of all available survey forms, keyed by form name.
// Used by the service layer to look up questins for a given form.
var AllForms = map[string][]Question{
	"authors_programm": AuthorsProgramm,
	"new_user":         NewUserForm,
	"contact":          ContactForm,
	"event":            EventForm,
	"dating_short":     DatingShortForm,
	"dating_full":      DatingFullForm,
	"portrait":         PortraitForm,
	"consult":          ConsultForm,
}

var FormEndings = map[string]string{
	"new_user":         "<b>Приятно познакомиться.</b> 🤍\n\nВам открыт доступ к пространству. Выберите направление, которое сейчас наиболее актуально.",
	"event":            "<b>Заявка принята.</b> 🥂\n\nНаталья лично рассматривает каждое обращение, чтобы сохранить качество окружения. Мы напишем вам, когда появится подходящее событие.",
	"dating_short":     "<b>Благодарим за ответы.</b> 🤍\n\nЭто поможет нам лучше понять ваш запрос. Если вы готовы к более глубокому разбору — рекомендуем заполнить полную анкету.",
	"dating_full":      "<b>Анкета получена.</b> 🙏\n\nНаталья изучит ваши ценности и видение союза. Мы свяжемся с вами в ближайшее время.",
	"portrait":         "<b>Запрос на портрет партнёра принят.</b> 🕯\n\nНаталья подготовит основу для вашей сессии, чтобы сделать её максимально точной и глубокой. Ожидайте сообщения.",
	"consult":          "<b>Заявка на личную работу отправлена.</b> ✉️\n\nБлагодарим за доверие. Мы свяжемся с вами, чтобы согласовать удобное время.",
	"contact":          "Спасибо! Теперь вы можете пользоваться всеми возможностями пространства.",
	"authors_programm": "<b>Заявка на программу принята.</b> 🤍\n\nНаталья свяжется с вами в ближайшее время.",
}

var AuthorsProgramm = []Question{
	{
		ID: "program_info",
		Text: "Авторская программа «Путь к своему мужчине» 🤍\n\n" +
			"60 дней личного и группового сопровождения по методу «Архитектура встречи».\n\n" +
			"Для женщин, которые хотят выйти из ожидания и начать мягко, красиво и системно создавать путь к встрече своего мужчины.\n\n" +
			"В программе будем работать с состоянием, образом, пониманием своего мужчины, сайтами знакомств, офлайн-средой, свиданиями и личной стратегией.\n\n" +
			"<b>Что будет внутри:</b>\n" +
			"— личная консультация;\n" +
			"— практики женского состояния;\n" +
			"— разбор образа и анкеты;\n" +
			"— стратегия знакомств;\n" +
			"— поддержка в женском круге.\n\n" +
			"<b>Для кого программа:</b>\n" +
			"— если вы хотите зрелых и тёплых отношений;\n" +
			"— устали всё тащить одной;\n" +
			"— хотите понять, где и как знакомиться;\n" +
			"— готовы делать реальные шаги к личной жизни.\n\n" +
			"<b>Старт:</b> 19 мая\n" +
			"<b>Длительность:</b> 60 дней\n" +
			"<b>Мест:</b> до 8 женщин\n\n" +
			"Хотите понять, подходит ли вам программа? Оставьте заявку 🤍",
		Options:  []string{"Оставить заявку", BtnToMainMenu},
		Type:     InputChoice,
		InfoOnly: true,
	},
}

// NewUserForm is the initial registration form shown to every new user.
var NewUserForm = []Question{
	{ID: "reg_name", Text: "Как к вам можно обращаться?", Type: InputText},
}

var ContactForm = []Question{
	{ID: "reg_phone", Text: "Оставьте ваш контактный номер", Type: InputPhone},
	{ID: "reg_birthdate", Text: "Укажите вашу дату рождения (ДД.ММ.ГГГГ)", Type: InputText},
	{ID: "reg_city", Text: "Укажите город проживания", Type: InputText},
	{ID: "reg_gender", Text: "Укажите ваш пол", Options: []string{"Женщина", "Мужчина"}, Type: InputChoice},
}

// EventForm collects information for users interested in matchmaking events.
var EventForm = []Question{
	{ID: "event_family_status", Text: "Ваш семейный статус?", Options: []string{"Не был(а) в браке", "В разводе", "Вдовец/Вдова", "В процессе расставания"}, Type: InputChoice},
	{ID: "event_is_repeat", Text: "Были ли вы раньше на наших мероприятиях?", Options: []string{"Да", "Нет"}, Type: InputChoice},
	{ID: "event_goal", Text: "Что вам сейчас ближе?", Options: []string{"Серьёзные отношения", "Знакомство", "Окружение", "Узнать формат"}, Type: InputChoice},
}

// DatingShortForm is the short partner-matching questionnaire.
// On completion, the user is offered to continue with DatingFullForm.
var DatingShortForm = []Question{
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
