package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// GetEnv retrieves the value of the environment variable named by envVar.
// It optionally loads a .env file if one exists (for local development),
// but gracefully falls back to system / container environment variables if .env is not found.
func GetEnv(envVar string) (string, error) {
	// Silently ignore error if .env file doesn't exist (standard for Docker / cloud deployments)
	_ = godotenv.Load(".env")

	token := os.Getenv(envVar)
	if token == "" {
		return "", fmt.Errorf("environment variable %s is not set", envVar)
	}
	return token, nil
}
