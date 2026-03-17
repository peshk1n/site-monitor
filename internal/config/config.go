package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUrl          string
	ServerPort     string
	TelegramToken  string
	TelegramChatID int64
}

func Load() *Config {
	godotenv.Load()

	chatID, _ := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)

	return &Config{
		DBUrl:          os.Getenv("DATABASE_URL"),
		ServerPort:     os.Getenv("SERVER_PORT"),
		TelegramToken:  os.Getenv("TELEGRAM_TOKEN"),
		TelegramChatID: chatID,
	}
}
