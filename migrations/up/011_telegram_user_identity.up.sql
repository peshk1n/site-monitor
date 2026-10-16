ALTER TABLE users
    ADD COLUMN telegram_user_id BIGINT,
    ADD CONSTRAINT users_telegram_user_key UNIQUE (telegram_user_id),
    ADD CONSTRAINT users_telegram_user_check CHECK (telegram_user_id > 0);
