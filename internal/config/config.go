// Пакет config содержит структуры конфигурации и функции загрузки настроек агента и сервера из флагов и переменных
// окружения.
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
	defaultStoreInterval  = 300
	defaultRestore        = false
	defaultRateLimit      = 10
)

// AgentConfig описывает конфигурацию агента.
//
// Значения берутся из флагов и переменных окружения в GetAgentConfig.
type AgentConfig struct {
	// ServerAddr — адрес сервера метрик в формате host:port.
	ServerAddr string
	// ReportInterval — периодичность отправки метрик на сервер (в секундах).
	ReportInterval int
	// PollInterval — периодичность опроса runtime-метрик (в секундах).
	PollInterval int
	// Key — ключ для подписи запросов (опционально).
	Key string
	// RateLimit — ограничение на число одновременно исходящих запросов агента.
	RateLimit int
}

// ServerConfig описывает конфигурацию сервера метрик.
//
// Значения берутся из флагов и переменных окружения в GetServerConfig.
type ServerConfig struct {
	// ServerAddr — адрес HTTP-сервера в формате host:port.
	ServerAddr string
	// StoreInterval — периодичность сохранения метрик на диск (в секундах).
	StoreInterval int
	// FileStoragePath — путь к файлу для хранения метрик (опционально).
	FileStoragePath string
	// Restore — признак необходимости восстановления метрик из файла при старте.
	Restore bool
	// DatabaseDSN — строка подключения к PostgreSQL (опционально).
	DatabaseDSN string
	// Key — ключ для подписи запросов (опционально).
	Key string
	// AuditFile — путь к файлу для записи сообщений событий аудита (опционально).
	AuditFile string
	// AuditURL — URL для отправки сообщений событий аудита по HTTP (опционально).
	AuditURL string
	// PprofAddr — адрес pprof-сервера в формате host:port (опционально).
	PprofAddr string
}

// GetServerConfig парсит флаги/переменные окружения и возвращает ServerConfig.
func GetServerConfig() (ServerConfig, error) {
	var serverAddrFlag string
	flag.Func("a", serverAddrUsage(), serverAddrFlagParser(&serverAddrFlag))
	storeIntervalFlag := flag.Int("i", defaultStoreInterval, "интервал сохранения метрик на диск")
	fileStoragePathFlag := flag.String("f", "", "путь до файла с метриками")
	restoreFlag := flag.Bool("r", defaultRestore, "восстанавливать метрики из файла при старте")
	databaseDSNFlag := flag.String("d", "", "строка подключения к БД")
	keyFlag := flag.String("k", "", "ключ для подписи запросов")
	auditFileFlag := flag.String("audit-file", "", "путь к файлу с логами аудита")
	auditURLFlag := flag.String("audit-url", "", "полный URL для отправки логов аудита")
	pprofAddrFlag := flag.String("pprof", "", "адрес pprof-сервера")
	flag.Parse()

	cfg := ServerConfig{}
	serverAddr, err := getServerAddr(serverAddrFlag)
	if err != nil {
		return cfg, err
	}
	storeInterval, err := getStoreInterval(storeIntervalFlag)
	if err != nil {
		return cfg, err
	}
	fileStoragePath := getFileStoragePath(fileStoragePathFlag)
	restore, err := getRestore(restoreFlag)
	if err != nil {
		return cfg, err
	}
	databaseDSN, err := getDatabaseDSN(databaseDSNFlag)
	if err != nil {
		return cfg, err
	}
	key, err := getKey(keyFlag)
	if err != nil {
		return cfg, err
	}
	auditFile, err := getAuditFile(auditFileFlag)
	if err != nil {
		return cfg, err
	}
	auditURL, err := getAuditURL(auditURLFlag)
	if err != nil {
		return cfg, err
	}
	pprofAddr := getPprofAddr(pprofAddrFlag)

	cfg.ServerAddr = serverAddr
	cfg.StoreInterval = storeInterval
	cfg.FileStoragePath = fileStoragePath
	cfg.Restore = restore
	cfg.DatabaseDSN = databaseDSN
	cfg.Key = key
	cfg.AuditFile = auditFile
	cfg.AuditURL = auditURL
	cfg.PprofAddr = pprofAddr
	return cfg, nil
}

func getStoreInterval(storeIntervalFlag *int) (int, error) {
	storeIntervalEnvStr, ok := os.LookupEnv("STORE_INTERVAL")
	if !ok {
		return *storeIntervalFlag, nil
	}
	storeIntervalEnv, err := strconv.Atoi(storeIntervalEnvStr)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to convert STORE_INTERNAL environment variable value '%s' to integer: %w",
			storeIntervalEnvStr,
			err,
		)
	}
	return storeIntervalEnv, nil
}

func getFileStoragePath(fileStoragePathFlag *string) string {
	fileStoragePathEnvStr, ok := os.LookupEnv("FILE_STORAGE_PATH")
	if !ok {
		return *fileStoragePathFlag
	}
	return fileStoragePathEnvStr
}

func getRestore(restoreFlag *bool) (bool, error) {
	restoreEnvStr, ok := os.LookupEnv("RESTORE")
	if !ok {
		return *restoreFlag, nil
	}
	restoreEnv, err := strconv.ParseBool(restoreEnvStr)
	if err != nil {
		return false, fmt.Errorf(
			"failed to convert RESTORE environment variable value '%s' to boolean: %w",
			restoreEnvStr,
			err,
		)
	}
	return restoreEnv, nil
}

func getDatabaseDSN(databaseDSNFlag *string) (string, error) {
	databaseDSNEnvStr, ok := os.LookupEnv("DATABASE_DSN")
	if !ok {
		return *databaseDSNFlag, nil
	}
	return databaseDSNEnvStr, nil
}

func getServerAddr(serverAddrFlag string) (string, error) {
	serverAddrEnv, ok := os.LookupEnv("ADDRESS")
	if !ok && serverAddrFlag != "" {
		return serverAddrFlag, nil
	}
	if !ok {
		return defaultServerAddr, nil
	}
	if err := validateServerAddr(serverAddrEnv); err != nil {
		return "", err
	}
	return serverAddrEnv, nil
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

// GetAgentConfig парсит флаги/переменные окружения и возвращает AgentConfig.
func GetAgentConfig() (AgentConfig, error) {
	var serverAddrFlag string
	flag.Func("a", serverAddrUsage(), serverAddrFlagParser(&serverAddrFlag))
	reportIntervalFlag := flag.Int("r", defaultReportInterval, "частота отправки метрик на сервер")
	pollIntervalFlag := flag.Int("p", defaultPollInterval, "частота опроса метрик из пакета runtime")
	keyFlag := flag.String("k", "", "ключ для подписи запросов")
	rateLimitFlag := flag.Int("l", defaultRateLimit, "количество одновременно исходящих запросов агента на сервер")
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
	key, err := getKey(keyFlag)
	if err != nil {
		return cfg, err
	}
	rateLimit, err := getRateLimit(rateLimitFlag)
	if err != nil {
		return cfg, err
	}

	cfg.ServerAddr = serverAddr
	cfg.ReportInterval = reportInterval
	cfg.PollInterval = pollInterval
	cfg.Key = key
	cfg.RateLimit = rateLimit
	return cfg, nil
}

func getReportInterval(reportIntervalFlag *int) (int, error) {
	reportIntervalEnvStr, ok := os.LookupEnv("REPORT_INTERVAL")
	if !ok {
		return *reportIntervalFlag, nil
	}
	reportIntervalEnv, err := strconv.Atoi(reportIntervalEnvStr)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to convert REPORT_INTERVAL environment variable value '%s' to integer: %w",
			reportIntervalEnvStr,
			err,
		)
	}
	return reportIntervalEnv, nil
}

func getPollInterval(pollIntervalFlag *int) (int, error) {
	pollIntervalEnvStr, ok := os.LookupEnv("POLL_INTERVAL")
	if !ok {
		return *pollIntervalFlag, nil
	}
	pollIntervalEnv, err := strconv.Atoi(pollIntervalEnvStr)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to convert POLL_INTERVAL environment variable value '%s' to integer: %w",
			pollIntervalEnvStr,
			err,
		)
	}
	return pollIntervalEnv, nil
}

func getKey(keyFlag *string) (string, error) {
	keyEnvStr, ok := os.LookupEnv("KEY")
	if !ok {
		return *keyFlag, nil
	}
	return keyEnvStr, nil
}

func getAuditFile(auditFileFlag *string) (string, error) {
	auditFileEnvStr, ok := os.LookupEnv("AUDIT_FILE")
	if !ok {
		return *auditFileFlag, nil
	}
	return auditFileEnvStr, nil
}

func getAuditURL(auditURLFlag *string) (string, error) {
	auditURLEnvStr, ok := os.LookupEnv("AUDIT_URL")
	if !ok {
		return *auditURLFlag, nil
	}
	return auditURLEnvStr, nil
}

func getPprofAddr(pprofAddrFlag *string) string {
	pprofAddrEnvStr, ok := os.LookupEnv("PPROF_ADDR")
	if !ok {
		return *pprofAddrFlag
	}
	return pprofAddrEnvStr
}

func getRateLimit(rateLimitFlag *int) (int, error) {
	rateLimitEnvStr, ok := os.LookupEnv("RATE_LIMIT")
	if !ok {
		return *rateLimitFlag, nil
	}
	rateLimitEnv, err := strconv.Atoi(rateLimitEnvStr)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to convert RATE_LIMIT environment variable value '%s' to integer: %w",
			rateLimitEnvStr,
			err,
		)
	}
	return rateLimitEnv, nil
}
