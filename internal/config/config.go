package config

import (
	"MeowSentinelBot/internal/utils"
	"fmt"
	"log"
)

type Config struct {
	Bot Bot `json:"bot"`
}

func MustNewConfig(filePath string) *Config {
	var cfg Config
	if err := utils.ParseJSONFromFile(filePath, &cfg); err != nil {
		log.Fatal(fmt.Errorf("fatal error while parsing config file %s: %w", filePath, err))
	}
	return &cfg
}

type Bot struct {
	Db       DB       `json:"db"`
	Telegram Telegram `json:"telegram"`
}
type DB struct {
	Controller string `json:"controller"`
	DSN        string `json:"dsn"`
	Path       string `json:"path"`
}

type Telegram struct {
	EnvVar string `json:"env_var"`
}
