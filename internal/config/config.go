package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultServerAddr     = "localhost:8080"
	defaultReportInterval = 10
	defaultPollInterval   = 2
)

type AgentConfig struct {
	ServerAddr     string
	ReportInterval int
	PollInterval   int
}
type ServerConfig struct {
	ServerAddr string
}

func GetServerConfig() (ServerConfig, error) {
	var serverAddrFlag string
	flag.Func("a", serverAddrUsage(), serverAddrFlagParser(&serverAddrFlag))
	flag.Parse()

	cfg := ServerConfig{}
	serverAddr, err := getServerAddr(serverAddrFlag)
	if err != nil {
		return cfg, err
	}

	cfg.ServerAddr = serverAddr
	return cfg, nil
}

func getServerAddr(serverAddrFlag string) (string, error) {
	serverAddrEnv, ok := os.LookupEnv("ADDRESS")
	if ok {
		if err := validateServerAddr(serverAddrEnv); err != nil {
			return "", err
		}
		return serverAddrEnv, nil
	} else if serverAddrFlag != "" {
		return serverAddrFlag, nil
	} else {
		return defaultServerAddr, nil
	}
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
	var serverAddrFlag string
	flag.Func("a", serverAddrUsage(), serverAddrFlagParser(&serverAddrFlag))
	reportIntervalFlag := flag.Int("r", defaultReportInterval, "частота отправки метрик на сервер")
	pollIntervalFlag := flag.Int("p", defaultPollInterval, "частота опроса метрик из пакета runtime")
	flag.Parse()

	cfg := AgentConfig{}
	serverAddr, err := getServerAddr(serverAddrFlag)
	if err != nil {
		return cfg, err
	}
	reportInterval, err := getReportInterval(reportIntervalFlag)
	if err != nil {
		return cfg, err
	}
	pollInterval, err := getPollInterval(pollIntervalFlag)
	if err != nil {
		return cfg, err
	}

	cfg.ServerAddr = serverAddr
	cfg.ReportInterval = reportInterval
	cfg.PollInterval = pollInterval
	return cfg, nil
}

func getReportInterval(reportIntervalFlag *int) (int, error) {
	reportIntervalEnvStr, ok := os.LookupEnv("REPORT_INTERVAL")
	if ok {
		reportIntervalEnv, err := strconv.Atoi(reportIntervalEnvStr)
		if err != nil {
			return 0, err
		}
		return reportIntervalEnv, nil
	} else {
		return *reportIntervalFlag, nil
	}
}

func getPollInterval(pollIntervalFlag *int) (int, error) {
	pollIntervalEnvStr, ok := os.LookupEnv("POLL_INTERVAL")
	if ok {
		pollIntervalEnv, err := strconv.Atoi(pollIntervalEnvStr)
		if err != nil {
			return 0, err
		}
		return pollIntervalEnv, nil
	} else {
		return *pollIntervalFlag, nil
	}
}
