package utils

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func GetEnv(envVar string) (string, error) {
	if err := godotenv.Load(".env"); err != nil {
		return "", fmt.Errorf("error loading .env file: %w", err)
	}

	token := os.Getenv(envVar)
	if token == "" {
		return "", errors.New("no telegram bot token provided")
	}
	return token, nil
}
