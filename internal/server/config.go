package server

import (
	"os"
	"strconv"
)

type Config struct {
	ListenAddr string

	Difficulty     byte
	ProofTokenSize int
}

func NewConfig() *Config {
	c := new(Config)

	c.ListenAddr = envOrDefault("LISTEN_ADDR", "0.0.0.0:9000")

	c.Difficulty = byte(envOrDefaultInt("DIFFICULTY", 23))
	c.ProofTokenSize = envOrDefaultInt("PROOF_TOKEN_SIZE", 64)

	return c
}

func envOrDefault(key, defaultValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return value
}

func envOrDefaultInt(key string, defaultValue int) int {
	value, _ := os.LookupEnv(key)
	if v, err := strconv.Atoi(value); err == nil {
		return v
	}
	return defaultValue
}
