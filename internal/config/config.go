package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DBURL         string
	ServerPort    string
	TelegramToken string
	Workers       int
	AuthMaxAge    time.Duration
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, errors.New("unable to load local environment")
	}
	c := Config{DBURL: os.Getenv("DATABASE_URL"), ServerPort: os.Getenv("SERVER_PORT"), TelegramToken: os.Getenv("TELEGRAM_TOKEN"), Workers: 4, AuthMaxAge: 15 * time.Minute}
	if c.ServerPort == "" {
		c.ServerPort = "8080"
	}
	port, err := strconv.Atoi(c.ServerPort)
	if err != nil || port < 1 || port > 65535 {
		return c, errors.New("invalid SERVER_PORT")
	}
	if c.DBURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if c.TelegramToken == "" {
		return c, errors.New("TELEGRAM_TOKEN is required for API authentication")
	}
	if raw := os.Getenv("CHECK_WORKERS"); raw != "" {
		c.Workers, err = strconv.Atoi(raw)
		if err != nil || c.Workers < 0 || c.Workers > 100 {
			return c, errors.New("CHECK_WORKERS must be between 0 and 100")
		}
	}
	return c, nil
}
