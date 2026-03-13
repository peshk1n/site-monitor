package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUrl      string
	ServerPort string
}

func Load() *Config {
	godotenv.Load()

	return &Config{
		DBUrl:      os.Getenv("DATABASE_URL"),
		ServerPort: os.Getenv("SERVER_PORT"),
	}
}
