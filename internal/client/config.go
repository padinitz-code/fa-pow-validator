package client

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerAddr string

	FetchWorkers int
	Timeout      time.Duration
}

func NewConfig() *Config {
	c := new(Config)

	c.ServerAddr = envOrDefault("SERVER_ADDR", "127.0.0.1:9000")

	c.FetchWorkers = envOrDefaultInt("FETCH_WORKERS", 4)

	msTimeout := envOrDefaultInt("TIMEOUT", 1000)
	c.Timeout = time.Millisecond * time.Duration(msTimeout)

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
