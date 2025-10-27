package config

import (
	"flag"
)

type Config struct {
	ServerAddr     *string
	ReportInterval *int
	PollInterval   *int
}

func ParseFlags() Config {
	config := Config{}
	config.ServerAddr = flag.String("a", "http://localhost:8080", "адрес HTTP-сервера")
	config.ReportInterval = flag.Int("r", 10, "частота отправки метрик на сервер")
	config.PollInterval = flag.Int("p", 2, "частота опроса метрик из пакета runtime")
	flag.Parse()
	return config
}
