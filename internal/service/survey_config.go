package service

type Question struct {
	ID       string
	Text     string
	Options  []string
	NextForm string
}

var AllForms = map[string][]Question{
	"new_user":     NewUserForm,
	"event":        EventForm,
	"dating_short": DatingShortForm,
	"dating_full":  DatingFullForm,
	"protrait":     PortraitForm,
	"consult":      ConsultForm,
}

var NewUserForm = []Question{
	{ID: "reg_name", Text: "Как к вам можно обращаться?"},
	{ID: "reg_phone", Text: "Оставьте ваш контактный номер"},
	{ID: "reg_birthdate", Text: "Укажите вашу дату рождения (ДД.ММ.ГГГГ)"},
}

var EventForm = []Question{
	{ID: "event_city", Text: "Ваш город?"},
	{ID: "event_gender", Text: "Ваш пол?", Options: []string{"Мужчина", "Женщина"}},
	{ID: "event_age", Text: "Ваш возраст?"},
	{ID: "event_family_status", Text: "Ваш семейный статус?", Options: []string{"Не был(а) в браке", "В разводе", "Вдовец/Вдова", "В процессе расставания"}},
	{ID: "event_is_repeat", Text: "Были ли вы раньше на наших мероприятиях?", Options: []string{"Да", "Нет"}},
	{ID: "event_goal", Text: "Что вам сейчас ближе?", Options: []string{"Серьёзные отношения", "Знакомство", "Окружение", "Узнать формат"}},
}

var DatingShortForm = []Question{
	{ID: "dating_short_age", Text: "Ваш возраст?"},
	{ID: "dating_short_city", Text: "Ваш город?"},
	{ID: "dating_short_gender", Text: "Ваш пол?", Options: []string{"Женщина", "Мужчина"}},
	{ID: "dating_short_family_status", Text: "Ваш семейный статус?", Options: []string{"Не был(а) в браке", "В разводе", "Вдовец/Вдова", "В отношениях"}},
	{ID: "dating_short_has_children", Text: "Есть ли у вас дети?", Options: []string{"Да", "Нет"}},
	{ID: "dating_short_format_goal", Text: "Какой формат отношений для вас ближе?", Options: []string{"Брак", "Отношения", "Знакомство", "Присматриваюсь"}},
	{ID: "dating_short_occupation", Text: "Чем вы занимаетесь?"},
	{ID: "dating_short_lifestyle", Text: "Какой у вас образ жизни?", Options: []string{"Активный", "Спокойный", "Смешанный", "В поездках"}},
	{ID: "dating_short_important_traits", Text: "Что для вас важно в партнёре?"},
	{ID: "dating_short_why_now", Text: "Почему именно сейчас вы открыты к отношениям?", NextForm: "DatingFullForm"},
}

var DatingFullForm = []Question{
	{ID: "dating_full_qualities", Text: "Назовите 5 важных качеств вашего партнёра"},
	{ID: "dating_full_unacceptable", Text: "Какие форматы отношений для вас неприемлемы?"},
	{ID: "dating_full_mature_union", Text: "Что для вас значит зрелый союз?"},
	{ID: "dating_full_self_in_relation", Text: "Как вы проявляетесь в отношениях, когда вам хорошо?"},
	{ID: "dating_full_give", Text: "Что вы готовы вкладывать в союз?"},
	{ID: "dating_full_receive", Text: "Что для вас важно получать от партнёра?"},
	{ID: "dating_full_past_patterns", Text: "Какие сценарии из прошлого вы не хотите повторять?"},
	{ID: "dating_full_readiness", Text: "Насколько вы сейчас открыты к реальным встречам?"},
	{ID: "dating_full_values", Text: "Какие ценности партнёра для вас обязательны?"},
	{ID: "dating_full_vision_3y", Text: "Опишите ваш идеальный союз через 3 года"},
	{ID: "dating_full_age_range", Text: "Какие возрастные рамки партнёра для вас комфортны?"},
	{ID: "dating_full_location", Text: "Какие локации проживания для вас подходят?"},
	{ID: "dating_full_status_importance", Text: "Насколько для вас важны статус и доход партнёра?"},
	{ID: "dating_full_past_blockers", Text: "Что чаще всего мешало отношениям складываться раньше?"},
	{ID: "dating_full_recognition", Text: "Как вы понимаете, что человек «ваш»?"},
}

var PortraitForm = []Question{
	{ID: "portrait_main_focus", Text: "Что для вас сейчас наиболее важно?", Options: []string{"Понять, кто мне подходит", "Почему не складываются отношения", "Повторяющийся сценарий", "Создать зрелый союз"}},
}

var ConsultForm = []Question{
	{ID: "consult_request", Text: "Опишите ваш запрос в свободной форме"},
}
