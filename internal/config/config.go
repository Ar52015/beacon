package config

import (
	"errors"
	"os"
)

type Config struct {
	Addr  string
	Token string
}

func Load() (Config, error) {
	addr := os.Getenv("BEACON_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	token := os.Getenv("BEACON_TOKEN")
	if token == "" {
		return Config{}, errors.New("BEACON_TOKEN cannot be empty")
	}

	return Config{Addr: addr, Token: token}, nil
}
