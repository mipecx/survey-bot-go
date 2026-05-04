CREATE TABLE IF NOT EXISTS users (
    tg_id        BIGINT PRIMARY KEY,
    username     TEXT,
    current_form TEXT DEFAULT 'new_user',
    current_step TEXT DEFAULT '',
    full_name    TEXT DEFAULT '',
    phone        TEXT DEFAULT '',
    birth_date   DATE,
    survey_data  JSONB DEFAULT '{}',
    created_at   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
