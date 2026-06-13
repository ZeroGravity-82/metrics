// Пакет config содержит структуры конфигурации и функции загрузки настроек агента и сервера из флагов и переменных
// окружения.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

const (
	defaultHTTPServerAddr = "localhost:8080"
	defaultGRPCServerAddr = "localhost:3201"
	defaultTLSCertPath    = "certs/server.crt"
	defaultTLSKeyPath     = "certs/server.key"
	defaultCACertPath     = "certs/ca.crt"
	defaultReportInterval = 10
	defaultPollInterval   = 2
	defaultStoreInterval  = 300
	defaultRestore        = false
	defaultRateLimit      = 10
)

// AgentConfig описывает конфигурацию агента.
//
// HTTPServerAddr - адрес HTTP-сервера метрик в формате host:port.
//
// GRPCServerAddr - адрес gRPC-сервера в формате host:port.
//
// CACertPath - путь к CA-сертификату для проверки TLS-сертификата сервера.
//
// ReportInterval - периодичность отправки метрик на сервер.
//
// PollInterval - периодичность опроса runtime-метрик.
//
// SignatureKey - ключ для подписи запросов (опционально).
//
// CryptoKeyPath - путь к публичному ключу для шифрования данных (опционально).
//
// RateLimit - ограничение на число одновременно исходящих запросов агента.
type AgentConfig struct {
	HTTPServerAddr string         `json:"address"`
	GRPCServerAddr string         `json:"grpc_address"`
	CACertPath     string         `json:"ca_cert"`
	ReportInterval configDuration `json:"report_interval"`
	PollInterval   configDuration `json:"poll_interval"`
	SignatureKey   string         `json:"signature_key"`
	CryptoKeyPath  string         `json:"crypto_key"`
	RateLimit      int            `json:"rate_limit"`
}

// ServerConfig описывает конфигурацию сервера метрик.
//
// HTTPServerAddr - адрес HTTP-сервера в формате host:port.
//
// GRPCServerAddr - адрес gRPC-сервера в формате host:port.
//
// TLSCertPath - путь к TLS-сертификату HTTP/gRPC-серверов.
//
// TLSKeyPath - путь к приватному TLS-ключу HTTP/gRPC-серверов.
//
// StoreInterval - периодичность сохранения метрик на диск.
//
// FileStoragePath - путь к файлу для хранения метрик (опционально).
//
// Restore - признак необходимости восстановления метрик из файла при старте.
//
// DatabaseDSN - строка подключения к PostgreSQL (опционально).
//
// SignatureKey - ключ для подписи запросов (опционально).
//
// CryptoKeyPath - путь к приватному ключу для дешифрования данных (опционально).
//
// AuditFile - путь к файлу для записи сообщений событий аудита (опционально).
//
// AuditURL - URL для отправки сообщений событий аудита по HTTP (опционально).
//
// PprofAddr - адрес pprof-сервера в формате host:port (опционально).
//
// TrustedSubnet - CIDR доверенной подсети (опционально).
type ServerConfig struct {
	HTTPServerAddr  string         `json:"address"`
	GRPCServerAddr  string         `json:"grpc_address"`
	TLSCertPath     string         `json:"tls_cert"`
	TLSKeyPath      string         `json:"tls_key"`
	StoreInterval   configDuration `json:"store_interval"`
	FileStoragePath string         `json:"store_file"`
	Restore         bool           `json:"restore"`
	DatabaseDSN     string         `json:"database_dsn"`
	SignatureKey    string         `json:"signature_key"`
	CryptoKeyPath   string         `json:"crypto_key"`
	AuditFile       string         `json:"audit_file"`
	AuditURL        string         `json:"audit_url"`
	PprofAddr       string         `json:"pprof_address"`
	TrustedSubnet   string         `json:"trusted_subnet"`
}

// GetServerConfig парсит файл конфигурации/флаги/переменные окружения и возвращает ServerConfig.
func GetServerConfig() (ServerConfig, error) {
	const (
		storeIntervalFlagName = "store-interval"
		restoreFlagName       = "restore"
	)

	configFileNameFlag := pflag.StringP("config", "c", "", "путь к файлу конфигурации")
	var httpServerAddrFlag string
	pflag.FuncP("address", "a", httpServerAddrUsage(), httpServerAddrFlagParser(&httpServerAddrFlag))
	var grpcServerAddrFlag string
	pflag.FuncP("grpc-address", "", grpcServerAddrUsage(), grpcServerAddrFlagParser(&grpcServerAddrFlag))
	tlsCertPathFlag := pflag.StringP("tls-cert", "", defaultTLSCertPath, "путь к TLS-сертификату HTTP/gRPC-серверов")
	tlsKeyPathFlag := pflag.StringP("tls-key", "", defaultTLSKeyPath, "путь к приватному TLS-ключу HTTP/gRPC-серверов")
	storeIntervalFlag := pflag.IntP(storeIntervalFlagName, "i", defaultStoreInterval, "интервал сохранения метрик на диск (в секундах)")
	fileStoragePathFlag := pflag.StringP("file-storage-path", "f", "", "путь до файла с метриками")
	restoreFlag := pflag.BoolP(restoreFlagName, "r", defaultRestore, "восстанавливать метрики из файла при старте")
	databaseDSNFlag := pflag.StringP("database-dsn", "d", "", "строка подключения к БД")
	signatureKeyFlag := pflag.StringP("signature-key", "k", "", "ключ для подписи запросов")
	cryptoKeyPathFlag := pflag.StringP("crypto-key", "", "", "путь к приватному ключу для дешифрования данных")
	auditFileFlag := pflag.StringP("audit-file", "", "", "путь к файлу с логами аудита")
	auditURLFlag := pflag.StringP("audit-url", "", "", "полный URL для отправки логов аудита")
	pprofAddrFlag := pflag.StringP("pprof", "", "", "адрес pprof-сервера")
	trustedSubnetFlag := pflag.StringP("trusted-subnet", "t", "", "CIDR доверенной подсети")
	pflag.Parse()

	cfg := ServerConfig{}
	cfgJSON, err := getConfigJSON[ServerConfig](configFileNameFlag)
	if err != nil {
		return cfg, err
	}
	httpServerAddr, err := getHTTPServerAddr(httpServerAddrFlag, cfgJSON.HTTPServerAddr)
	if err != nil {
		return cfg, err
	}
	grpcServerAddr, err := getGRPCServerAddr(grpcServerAddrFlag, cfgJSON.GRPCServerAddr)
	if err != nil {
		return cfg, err
	}
	tlsCertPath := getTLSCertPath(*tlsCertPathFlag, pflag.Lookup("tls-cert").Changed, cfgJSON.TLSCertPath)
	tlsKeyPath := getTLSKeyPath(*tlsKeyPathFlag, pflag.Lookup("tls-key").Changed, cfgJSON.TLSKeyPath)
	storeInterval, err := getStoreInterval(*storeIntervalFlag, pflag.Lookup(storeIntervalFlagName).Changed, cfgJSON.StoreInterval)
	if err != nil {
		return cfg, err
	}
	fileStoragePath := getFileStoragePath(*fileStoragePathFlag, cfgJSON.FileStoragePath)
	restore, err := getRestore(*restoreFlag, pflag.Lookup(restoreFlagName).Changed, cfgJSON.Restore)
	if err != nil {
		return cfg, err
	}
	databaseDSN, err := getDatabaseDSN(*databaseDSNFlag, cfgJSON.DatabaseDSN)
	if err != nil {
		return cfg, err
	}
	signatureKey := getSignatureKey(*signatureKeyFlag, cfgJSON.SignatureKey)
	cryptoKeyPath := getCryptoKeyPath(*cryptoKeyPathFlag, cfgJSON.CryptoKeyPath)
	auditFile := getAuditFile(*auditFileFlag, cfgJSON.AuditFile)
	auditURL := getAuditURL(*auditURLFlag, cfgJSON.AuditURL)
	pprofAddr := getPprofAddr(*pprofAddrFlag, cfgJSON.PprofAddr)
	trustedSubnet, err := getTrustedSubnet(*trustedSubnetFlag, cfgJSON.TrustedSubnet)
	if err != nil {
		return cfg, err
	}

	cfg.HTTPServerAddr = httpServerAddr
	cfg.GRPCServerAddr = grpcServerAddr
	cfg.TLSCertPath = tlsCertPath
	cfg.TLSKeyPath = tlsKeyPath
	cfg.StoreInterval = storeInterval
	cfg.FileStoragePath = fileStoragePath
	cfg.Restore = restore
	cfg.DatabaseDSN = databaseDSN
	cfg.SignatureKey = signatureKey
	cfg.CryptoKeyPath = cryptoKeyPath
	cfg.AuditFile = auditFile
	cfg.AuditURL = auditURL
	cfg.PprofAddr = pprofAddr
	cfg.TrustedSubnet = trustedSubnet
	return cfg, nil
}

func getConfigJSON[T any](configFileNameFlag *string) (T, error) {
	var cfgJSON T
	configFileName := getConfigFileName(configFileNameFlag)
	if configFileName != "" {
		data, err := os.ReadFile(configFileName)
		if err != nil {
			return cfgJSON, fmt.Errorf("failed to open config file %q: %w", configFileName, err)
		}
		if err = json.Unmarshal(data, &cfgJSON); err != nil {
			return cfgJSON, fmt.Errorf("failed to unmarshal config file %q: %w", configFileName, err)
		}
	}
	return cfgJSON, nil
}

func getConfigFileName(configFlag *string) string {
	configEnvStr, ok := os.LookupEnv("CONFIG")
	if !ok {
		return *configFlag
	}
	return configEnvStr
}

func getStoreInterval(
	storeIntervalFlag int,
	storeIntervalFlagChanged bool,
	storeIntervalCfgJSON configDuration,
) (configDuration, error) {
	storeIntervalEnvStr, ok := os.LookupEnv("STORE_INTERVAL")
	if ok {
		storeIntervalEnv, err := strconv.Atoi(storeIntervalEnvStr)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to convert STORE_INTERNAL environment variable value %q to integer: %w",
				storeIntervalEnvStr,
				err,
			)
		}
		return convertIntToConfigDuration(storeIntervalEnv), nil
	}
	if storeIntervalFlagChanged {
		return convertIntToConfigDuration(storeIntervalFlag), nil
	}
	if storeIntervalCfgJSON > 0 {
		return storeIntervalCfgJSON, nil
	}
	return convertIntToConfigDuration(storeIntervalFlag), nil
}
func convertIntToConfigDuration(seconds int) configDuration {
	return configDuration(time.Duration(seconds) * time.Second)
}

func getFileStoragePath(fileStoragePathFlag, fileStoragePathCfgJSON string) string {
	fileStoragePathEnvStr, ok := os.LookupEnv("FILE_STORAGE_PATH")
	if ok {
		return fileStoragePathEnvStr
	}
	if fileStoragePathFlag != "" {
		return fileStoragePathFlag
	}
	return fileStoragePathCfgJSON
}

func getRestore(restoreFlag, restoreFlagChanged, restoreCfgJSON bool) (bool, error) {
	restoreEnvStr, ok := os.LookupEnv("RESTORE")
	if ok {
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
	if restoreFlagChanged {
		return restoreFlag, nil
	}
	return restoreCfgJSON, nil
}

func getDatabaseDSN(databaseDSNFlag, databaseDSNCfgJSON string) (string, error) {
	databaseDSNEnvStr, ok := os.LookupEnv("DATABASE_DSN")
	if ok {
		return databaseDSNEnvStr, nil
	}
	if databaseDSNFlag != "" {
		return databaseDSNFlag, nil
	}
	return databaseDSNCfgJSON, nil
}

func getHTTPServerAddr(httpServerAddrFlag, httpServerAddrCfgJSON string) (string, error) {
	httpServerAddrEnvStr, ok := os.LookupEnv("ADDRESS")
	if !ok && httpServerAddrFlag != "" {
		if err := validateServerAddr(httpServerAddrFlag); err != nil {
			return "", err
		}
		return httpServerAddrFlag, nil
	}
	if !ok && httpServerAddrCfgJSON != "" {
		if err := validateServerAddr(httpServerAddrCfgJSON); err != nil {
			return "", err
		}
		return httpServerAddrCfgJSON, nil
	}
	if !ok {
		return defaultHTTPServerAddr, nil
	}
	if err := validateServerAddr(httpServerAddrEnvStr); err != nil {
		return "", err
	}
	return httpServerAddrEnvStr, nil
}

func getGRPCServerAddr(grpcServerAddrFlag, grpcServerAddrCfgJSON string) (string, error) {
	grpcServerAddrEnvStr, ok := os.LookupEnv("GRPC_ADDRESS")
	if !ok && grpcServerAddrFlag != "" {
		if err := validateServerAddr(grpcServerAddrFlag); err != nil {
			return "", err
		}
		return grpcServerAddrFlag, nil
	}
	if !ok && grpcServerAddrCfgJSON != "" {
		if err := validateServerAddr(grpcServerAddrCfgJSON); err != nil {
			return "", err
		}
		return grpcServerAddrCfgJSON, nil
	}
	if !ok {
		return defaultGRPCServerAddr, nil
	}
	if err := validateServerAddr(grpcServerAddrEnvStr); err != nil {
		return "", err
	}
	return grpcServerAddrEnvStr, nil
}

func httpServerAddrUsage() string {
	return fmt.Sprintf(`адрес HTTP-сервера (default "%s")`, defaultHTTPServerAddr)
}

func grpcServerAddrUsage() string {
	return fmt.Sprintf(`адрес gRPC-сервера (default "%s")`, defaultGRPCServerAddr)
}

func agentGRPCServerAddrUsage() string {
	return `адрес gRPC-сервера (default "")`
}

func httpServerAddrFlagParser(httpServerAddr *string) func(string) error {
	return func(flagValue string) error {
		if err := validateServerAddr(flagValue); err != nil {
			return err
		}
		*httpServerAddr = flagValue
		return nil
	}
}

func grpcServerAddrFlagParser(grpcServerAddr *string) func(string) error {
	return func(flagValue string) error {
		if flagValue == "" {
			*grpcServerAddr = flagValue
			return nil
		}
		if err := validateServerAddr(flagValue); err != nil {
			return err
		}
		*grpcServerAddr = flagValue
		return nil
	}
}

func validateServerAddr(v string) error {
	hp := strings.Split(v, ":")
	if len(hp) != 2 {
		return errors.New("server address must be in the format host:post (without specifying a scheme)")
	}
	return nil
}

// getAgentGRPCServerAddr возвращает пустую строку, если gRPC-адрес агента не задан явно.
//
// Для агента пустой gRPC-адрес означает, что gRPC-отправка выключена и используется HTTP.
func getAgentGRPCServerAddr(grpcServerAddrFlag, grpcServerAddrCfgJSON string) (string, error) {
	grpcServerAddrEnvStr, ok := os.LookupEnv("GRPC_ADDRESS")
	if !ok && grpcServerAddrFlag != "" {
		if err := validateServerAddr(grpcServerAddrFlag); err != nil {
			return "", err
		}
		return grpcServerAddrFlag, nil
	}
	if !ok && grpcServerAddrCfgJSON != "" {
		if err := validateServerAddr(grpcServerAddrCfgJSON); err != nil {
			return "", err
		}
		return grpcServerAddrCfgJSON, nil
	}
	if !ok || grpcServerAddrEnvStr == "" {
		return "", nil
	}
	if err := validateServerAddr(grpcServerAddrEnvStr); err != nil {
		return "", err
	}
	return grpcServerAddrEnvStr, nil
}

// GetAgentConfig парсит файл конфигурации/флаги/переменные окружения и возвращает AgentConfig.
func GetAgentConfig() (AgentConfig, error) {
	const (
		caCertFlagName         = "ca-cert"
		reportIntervalFlagName = "report-interval"
		pollIntervalFlagName   = "poll-interval"
		rateLimitFlagName      = "rate-limit"
	)

	configFileNameFlag := pflag.StringP("config", "c", "", "путь к файлу конфигурации")
	var httpServerAddrFlag string
	pflag.FuncP("address", "a", httpServerAddrUsage(), httpServerAddrFlagParser(&httpServerAddrFlag))
	var grpcServerAddrFlag string
	pflag.FuncP("grpc-address", "", agentGRPCServerAddrUsage(), grpcServerAddrFlagParser(&grpcServerAddrFlag))
	caCertPathFlag := pflag.StringP(caCertFlagName, "", defaultCACertPath, "путь к CA-сертификату для проверки TLS-сертификата сервера")
	reportIntervalFlag := pflag.IntP(reportIntervalFlagName, "r", defaultReportInterval, "частота отправки метрик на сервер (в секундах)")
	pollIntervalFlag := pflag.IntP(pollIntervalFlagName, "p", defaultPollInterval, "частота опроса метрик из пакета runtime (в секундах)")
	signatureKeyFlag := pflag.StringP("signature-key", "k", "", "ключ для подписи запросов")
	cryptoKeyPathFlag := pflag.StringP("crypto-key", "", "", "путь к публичному ключу для шифрования данных")
	rateLimitFlag := pflag.IntP(rateLimitFlagName, "l", defaultRateLimit, "количество одновременно исходящих запросов агента на сервер")
	pflag.Parse()

	cfg := AgentConfig{}
	cfgJSON, err := getConfigJSON[AgentConfig](configFileNameFlag)
	if err != nil {
		return cfg, err
	}
	httpServerAddr, err := getHTTPServerAddr(httpServerAddrFlag, cfgJSON.HTTPServerAddr)
	if err != nil {
		return cfg, err
	}
	grpcServerAddr, err := getAgentGRPCServerAddr(grpcServerAddrFlag, cfgJSON.GRPCServerAddr)
	if err != nil {
		return cfg, err
	}
	caCertPath := getCACertPath(*caCertPathFlag, pflag.Lookup(caCertFlagName).Changed, cfgJSON.CACertPath)
	reportInterval, err := getReportInterval(*reportIntervalFlag, pflag.Lookup(reportIntervalFlagName).Changed, cfgJSON.ReportInterval)
	if err != nil {
		return cfg, err
	}
	pollInterval, err := getPollInterval(*pollIntervalFlag, pflag.Lookup(pollIntervalFlagName).Changed, cfgJSON.PollInterval)
	if err != nil {
		return cfg, err
	}
	signatureKey := getSignatureKey(*signatureKeyFlag, cfgJSON.SignatureKey)
	cryptoKeyPath := getCryptoKeyPath(*cryptoKeyPathFlag, cfgJSON.CryptoKeyPath)
	rateLimit, err := getRateLimit(*rateLimitFlag, pflag.Lookup(rateLimitFlagName).Changed, cfgJSON.RateLimit)
	if err != nil {
		return cfg, err
	}

	cfg.HTTPServerAddr = httpServerAddr
	cfg.GRPCServerAddr = grpcServerAddr
	cfg.CACertPath = caCertPath
	cfg.ReportInterval = reportInterval
	cfg.PollInterval = pollInterval
	cfg.SignatureKey = signatureKey
	cfg.CryptoKeyPath = cryptoKeyPath
	cfg.RateLimit = rateLimit
	return cfg, nil
}

func getReportInterval(
	reportIntervalFlag int,
	reportIntervalFlagChanged bool,
	reportIntervalCfgJSON configDuration,
) (configDuration, error) {
	reportIntervalEnvStr, ok := os.LookupEnv("REPORT_INTERVAL")
	if ok {
		reportIntervalEnv, err := strconv.Atoi(reportIntervalEnvStr)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to convert REPORT_INTERVAL environment variable value '%s' to integer: %w",
				reportIntervalEnvStr,
				err,
			)
		}
		return convertIntToConfigDuration(reportIntervalEnv), nil
	}
	if reportIntervalFlagChanged {
		return convertIntToConfigDuration(reportIntervalFlag), nil
	}
	if reportIntervalCfgJSON > 0 {
		return reportIntervalCfgJSON, nil
	}
	return convertIntToConfigDuration(reportIntervalFlag), nil
}

func getPollInterval(
	pollIntervalFlag int,
	pollIntervalFlagChanged bool,
	pollIntervalCfgJSON configDuration,
) (configDuration, error) {
	pollIntervalEnvStr, ok := os.LookupEnv("POLL_INTERVAL")
	if ok {
		pollIntervalEnv, err := strconv.Atoi(pollIntervalEnvStr)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to convert POLL_INTERVAL environment variable value '%s' to integer: %w",
				pollIntervalEnvStr,
				err,
			)
		}
		return convertIntToConfigDuration(pollIntervalEnv), nil
	}
	if pollIntervalFlagChanged {
		return convertIntToConfigDuration(pollIntervalFlag), nil
	}
	if pollIntervalCfgJSON > 0 {
		return pollIntervalCfgJSON, nil
	}
	return convertIntToConfigDuration(pollIntervalFlag), nil
}

func getSignatureKey(signatureKeyFlag, signatureKeyCfgJSON string) string {
	signatureKeyEnvStr, ok := os.LookupEnv("KEY")
	if ok {
		return signatureKeyEnvStr
	}
	if signatureKeyFlag != "" {
		return signatureKeyFlag
	}
	return signatureKeyCfgJSON
}

func getCryptoKeyPath(cryptoKeyPathFlag, cryptoKeyPathCfgJSON string) string {
	cryptoKeyPathEnvStr, ok := os.LookupEnv("CRYPTO_KEY")
	if ok {
		return cryptoKeyPathEnvStr
	}
	if cryptoKeyPathFlag != "" {
		return cryptoKeyPathFlag
	}
	return cryptoKeyPathCfgJSON
}

func getTLSCertPath(tlsCertPathFlag string, tlsCertPathFlagChanged bool, tlsCertPathCfgJSON string) string {
	tlsCertPathEnvStr, ok := os.LookupEnv("TLS_CERT")
	if ok {
		return tlsCertPathEnvStr
	}
	if tlsCertPathFlagChanged {
		return tlsCertPathFlag
	}
	if tlsCertPathCfgJSON != "" {
		return tlsCertPathCfgJSON
	}
	return defaultTLSCertPath
}

func getTLSKeyPath(tlsKeyPathFlag string, tlsKeyPathFlagChanged bool, tlsKeyPathCfgJSON string) string {
	tlsKeyPathEnvStr, ok := os.LookupEnv("TLS_KEY")
	if ok {
		return tlsKeyPathEnvStr
	}
	if tlsKeyPathFlagChanged {
		return tlsKeyPathFlag
	}
	if tlsKeyPathCfgJSON != "" {
		return tlsKeyPathCfgJSON
	}
	return defaultTLSKeyPath
}

func getCACertPath(caCertPathFlag string, caCertPathFlagChanged bool, caCertPathCfgJSON string) string {
	caCertPathEnvStr, ok := os.LookupEnv("CA_CERT")
	if ok {
		return caCertPathEnvStr
	}
	if caCertPathFlagChanged {
		return caCertPathFlag
	}
	if caCertPathCfgJSON != "" {
		return caCertPathCfgJSON
	}
	return defaultCACertPath
}

func getAuditFile(auditFileFlag, auditFileCfgJSON string) string {
	auditFileEnvStr, ok := os.LookupEnv("AUDIT_FILE")
	if ok {
		return auditFileEnvStr
	}
	if auditFileFlag != "" {
		return auditFileFlag
	}
	return auditFileCfgJSON
}

func getAuditURL(auditURLFlag, auditURLCfgJSON string) string {
	auditURLEnvStr, ok := os.LookupEnv("AUDIT_URL")
	if ok {
		return auditURLEnvStr
	}
	if auditURLFlag != "" {
		return auditURLFlag
	}
	return auditURLCfgJSON
}

func getPprofAddr(pprofAddrFlag, pprofAddrCfgJSON string) string {
	pprofAddrEnvStr, ok := os.LookupEnv("PPROF_ADDR")
	if ok {
		return pprofAddrEnvStr
	}
	if pprofAddrFlag != "" {
		return pprofAddrFlag
	}
	return pprofAddrCfgJSON
}

func getTrustedSubnet(trustedSubnetFlag, trustedSubnetCfgJSON string) (string, error) {
	trustedSubnetEnvStr, ok := os.LookupEnv("TRUSTED_SUBNET")
	if !ok && trustedSubnetFlag != "" {
		if err := validateTrustedSubnet(trustedSubnetFlag); err != nil {
			return "", err
		}
		return trustedSubnetFlag, nil
	}
	if !ok && trustedSubnetCfgJSON != "" {
		if err := validateTrustedSubnet(trustedSubnetCfgJSON); err != nil {
			return "", err
		}
		return trustedSubnetCfgJSON, nil
	}
	if !ok {
		return "", nil
	}
	if trustedSubnetEnvStr == "" {
		return "", nil
	}
	if err := validateTrustedSubnet(trustedSubnetEnvStr); err != nil {
		return "", err
	}
	return trustedSubnetEnvStr, nil
}

func validateTrustedSubnet(v string) error {
	_, _, err := net.ParseCIDR(v)
	if err != nil {
		return fmt.Errorf("trusted subnet must be in the format CIDR: %w", err)
	}
	return nil
}

func getRateLimit(rateLimitFlag int, rateLimitFlagChanged bool, rateLimitCfgJSON int) (int, error) {
	rateLimitEnvStr, ok := os.LookupEnv("RATE_LIMIT")
	if ok {
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
	if rateLimitFlagChanged {
		return rateLimitFlag, nil
	}
	if rateLimitCfgJSON > 0 {
		return rateLimitCfgJSON, nil
	}
	return rateLimitFlag, nil
}
