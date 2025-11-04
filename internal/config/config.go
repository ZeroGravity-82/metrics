package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var defaultServerAddr = "localhost:8080"

type AgentConfig struct {
	ServerAddr     string
	ReportInterval int
	PollInterval   int
}
type ServerConfig struct {
	ServerAddr string
}

func GetServerConfig() (ServerConfig, error) {
	cfg := ServerConfig{}
	var serverAddr string

	var serverAddrFlag = &defaultServerAddr
	flag.Func("a", serverAddrUsage(), serverAddrFlagParser(serverAddrFlag))
	flag.Parse()

	serverAddrEnv, ok := os.LookupEnv("ADDRESS")
	if ok {
		if err := validateServerAddr(serverAddrEnv); err != nil {
			return cfg, err
		}
		serverAddr = serverAddrEnv
	} else {
		serverAddr = *serverAddrFlag
	}

	cfg.ServerAddr = serverAddr
	return cfg, nil
}

func serverAddrUsage() string {
	return fmt.Sprintf(`адрес HTTP-сервера (default "%s")`, defaultServerAddr)
}

func serverAddrFlagParser(serverAddr *string) func(string) error {
	return func(flagValue string) error {
		if err := validateServerAddr(flagValue); err != nil {
			return err
		}
		*serverAddr = flagValue
		return nil
	}
}

func validateServerAddr(v string) error {
	hp := strings.Split(v, ":")
	if len(hp) != 2 {
		return errors.New("адрес сервера должен быть в формате host:port (без указания схемы)")
	}
	return nil
}

func GetAgentConfig() (AgentConfig, error) {
	cfg := AgentConfig{}
	var serverAddr string
	var reportInterval, pollInterval int

	var serverAddrFlag = &defaultServerAddr
	flag.Func("a", serverAddrUsage(), serverAddrFlagParser(serverAddrFlag))
	reportIntervalFlag := flag.Int("r", 10, "частота отправки метрик на сервер")
	pollIntervalFlag := flag.Int("p", 2, "частота опроса метрик из пакета runtime")
	flag.Parse()

	serverAddrEnv, ok := os.LookupEnv("ADDRESS")
	if ok {
		if err := validateServerAddr(serverAddrEnv); err != nil {
			return cfg, err
		}
		serverAddr = serverAddrEnv
	} else {
		serverAddr = *serverAddrFlag
	}

	reportIntervalEnvStr, ok := os.LookupEnv("REPORT_INTERVAL")
	if ok {
		reportIntervalEnv, err := strconv.Atoi(reportIntervalEnvStr)
		if err != nil {
			return cfg, err
		}
		reportInterval = reportIntervalEnv
	} else {
		reportInterval = *reportIntervalFlag
	}

	pollIntervalEnvStr, ok := os.LookupEnv("POLL_INTERVAL")
	if ok {
		pollIntervalEnv, err := strconv.Atoi(pollIntervalEnvStr)
		if err != nil {
			return cfg, err
		}
		pollInterval = pollIntervalEnv
	} else {
		pollInterval = *pollIntervalFlag
	}

	cfg.ServerAddr = serverAddr
	cfg.ReportInterval = reportInterval
	cfg.PollInterval = pollInterval
	return cfg, nil
}
