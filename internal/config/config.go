package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUrl          string
	ServerPort     string
	TelegramToken  string
	TelegramChatID string
}

func Load() *Config {
	godotenv.Load()

	return &Config{
		DBUrl:          os.Getenv("DATABASE_URL"),
		ServerPort:     os.Getenv("SERVER_PORT"),
		TelegramToken:  os.Getenv("TELEGRAM_TOKEN"),
		TelegramChatID: os.Getenv("TELEGRAM_CHAT_ID"),
	}
}
