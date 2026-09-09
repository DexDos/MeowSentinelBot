package config

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	Bot Bot `json:"bot"`
}

func MustNewConfig(filePath string) *Config {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Fatal while opening the config file: ", err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatal("Fatal while closing the config file: ", err)
		}
	}(f)
	var cfg Config
	err = json.NewDecoder(f).Decode(&cfg)
	if err != nil {
		log.Fatal("Fatal while parsing the config file: ", err)
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
