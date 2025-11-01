package config

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

var defaultServerAddr string = "localhost:8080"

type Config struct {
	ServerAddr     *string
	ReportInterval *int
	PollInterval   *int
}

func ParseFlags() Config {
	config := Config{
		ServerAddr: &defaultServerAddr,
	}

	flag.Func("a", serverAddrUsage(), serverAddrParser(&config))
	config.ReportInterval = flag.Int("r", 10, "частота отправки метрик на сервер")
	config.PollInterval = flag.Int("p", 2, "частота опроса метрик из пакета runtime")
	flag.Parse()

	return config
}

func serverAddrUsage() string {
	return fmt.Sprintf(`адрес HTTP-сервера (default "%s")`, defaultServerAddr)
}

func serverAddrParser(config *Config) func(string) error {
	return func(flagValue string) error {
		if flagValue == "" {
			config.ServerAddr = &defaultServerAddr
			return nil
		}
		hp := strings.Split(flagValue, ":")
		if len(hp) != 2 {
			return errors.New("адрес сервера должен быть в формате host:port (без указания схемы)")
		}
		config.ServerAddr = &flagValue
		return nil
	}
}
