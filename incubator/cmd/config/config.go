package config

import "content-automation-engine/internal/clock"

type Config struct {
	Clock clock.Clock
}

func NewConfig() *Config {
	return &Config{
		Clock: clock.NewRealClock(),
	}
}
