package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanGetServerConfig_Default проверяет поведение по умолчанию, когда ни файл конфигурации, ни флаги, ни
// переменные окружения агента не заданы.
func TestCanGetServerConfig_Default(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"server"}
	err := os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("GRPC_ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_CERT")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("STORE_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("FILE_STORAGE_PATH")
	require.NoError(t, err)
	err = os.Unsetenv("RESTORE")
	require.NoError(t, err)
	err = os.Unsetenv("DATABASE_DSN")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_FILE")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_URL")
	require.NoError(t, err)
	err = os.Unsetenv("CRYPTO_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("PPROF_ADDR")
	require.NoError(t, err)
	err = os.Unsetenv("TRUSTED_SUBNET")
	require.NoError(t, err)
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, defaultHTTPServerAddr, cfg.HTTPServerAddr)
	assert.Equal(t, defaultGRPCServerAddr, cfg.GRPCServerAddr)
	assert.Equal(t, defaultTLSCertPath, cfg.TLSCertPath)
	assert.Equal(t, defaultTLSKeyPath, cfg.TLSKeyPath)
	assert.Equal(t, convertIntToConfigDuration(defaultStoreInterval), cfg.StoreInterval)
	assert.Equal(t, "", cfg.FileStoragePath)
	assert.Equal(t, defaultRestore, cfg.Restore)
	assert.Equal(t, "", cfg.DatabaseDSN)
	assert.Equal(t, "", cfg.SignatureKey)
	assert.Equal(t, "", cfg.AuditFile)
	assert.Equal(t, "", cfg.AuditURL)
	assert.Equal(t, "", cfg.CryptoKeyPath)
	assert.Equal(t, "", cfg.PprofAddr)
	assert.Equal(t, "", cfg.TrustedSubnet)
}

// TestCanGetServerConfig_JSON_FromFlag проверяет парсинг файла конфигурации, имя которого было передано через флаг.
func TestCanGetServerConfig_JSON_FromFlag(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	configPath := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(configPath, []byte(`
{
  "address":"localhost:9090",
  "grpc_address":"localhost:3202",
  "tls_cert": "server.crt",
  "tls_key": "server.key",
  "store_interval":"300s",
  "store_file":"server-metrics.json",
  "restore":true,
  "database_dsn":"postgres://metrics",
  "signature_key": "secret",
  "audit_file": "audit.json",
  "audit_url": "https://audit.site/hook",
  "crypto_key":"server-private.pem",
  "pprof_address": "localhost:6060",
  "trusted_subnet": "192.168.0.1/24"
}
`), 0o600)
	require.NoError(t, err)
	os.Args = []string{"server", "-c", configPath}

	err = os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("GRPC_ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_CERT")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("STORE_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("FILE_STORAGE_PATH")
	require.NoError(t, err)
	err = os.Unsetenv("RESTORE")
	require.NoError(t, err)
	err = os.Unsetenv("DATABASE_DSN")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_FILE")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_URL")
	require.NoError(t, err)
	err = os.Unsetenv("CRYPTO_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("PPROF_ADDR")
	require.NoError(t, err)
	err = os.Unsetenv("TRUSTED_SUBNET")
	require.NoError(t, err)
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:9090", cfg.HTTPServerAddr)
	assert.Equal(t, "localhost:3202", cfg.GRPCServerAddr)
	assert.Equal(t, "server.crt", cfg.TLSCertPath)
	assert.Equal(t, "server.key", cfg.TLSKeyPath)
	assert.Equal(t, configDuration(300*time.Second), cfg.StoreInterval)
	assert.Equal(t, "server-metrics.json", cfg.FileStoragePath)
	assert.Equal(t, true, cfg.Restore)
	assert.Equal(t, "postgres://metrics", cfg.DatabaseDSN)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, "audit.json", cfg.AuditFile)
	assert.Equal(t, "https://audit.site/hook", cfg.AuditURL)
	assert.Equal(t, "server-private.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "localhost:6060", cfg.PprofAddr)
	assert.Equal(t, "192.168.0.1/24", cfg.TrustedSubnet)
}

// TestCanGetServerConfig_JSON_FromEnv проверяет парсинг файла конфигурации, имя которого было передано через переменную
// окружения.
func TestCanGetServerConfig_JSON_FromEnv(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	configPath := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(configPath, []byte(`
{
  "address":"localhost:9090",
  "grpc_address":"localhost:3202",
  "tls_cert": "server.crt",
  "tls_key": "server.key",
  "store_interval":"300s",
  "store_file":"server-metrics.json",
  "restore":true,
  "database_dsn":"postgres://metrics",
  "signature_key": "secret",
  "audit_file": "audit.json",
  "audit_url": "https://audit.site/hook",
  "crypto_key":"server-private.pem",
  "pprof_address": "localhost:6060",
  "trusted_subnet": "192.168.0.1/24"
}
`), 0o600)
	require.NoError(t, err)

	err = os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("GRPC_ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_CERT")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("STORE_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("FILE_STORAGE_PATH")
	require.NoError(t, err)
	err = os.Unsetenv("RESTORE")
	require.NoError(t, err)
	err = os.Unsetenv("DATABASE_DSN")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_FILE")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_URL")
	require.NoError(t, err)
	err = os.Unsetenv("CRYPTO_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("PPROF_ADDR")
	require.NoError(t, err)
	err = os.Unsetenv("TRUSTED_SUBNET")
	require.NoError(t, err)
	err = os.Setenv("CONFIG", configPath)
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:9090", cfg.HTTPServerAddr)
	assert.Equal(t, "localhost:3202", cfg.GRPCServerAddr)
	assert.Equal(t, "server.crt", cfg.TLSCertPath)
	assert.Equal(t, "server.key", cfg.TLSKeyPath)
	assert.Equal(t, configDuration(300*time.Second), cfg.StoreInterval)
	assert.Equal(t, "server-metrics.json", cfg.FileStoragePath)
	assert.Equal(t, true, cfg.Restore)
	assert.Equal(t, "postgres://metrics", cfg.DatabaseDSN)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, "audit.json", cfg.AuditFile)
	assert.Equal(t, "https://audit.site/hook", cfg.AuditURL)
	assert.Equal(t, "server-private.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "localhost:6060", cfg.PprofAddr)
	assert.Equal(t, "192.168.0.1/24", cfg.TrustedSubnet)
}

// TestCanGetServerConfig_Flag проверяет парсинг параметров командной строки сервера.
func TestCanGetServerConfig_Flag(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"server",
		"-a=127.0.0.1:8081",
		"--grpc-address=127.0.0.1:3202",
		"--tls-cert=server-flag.crt",
		"--tls-key=server-flag.key",
		"-i=200",
		"-f=metrics.json",
		"-r",
		"-d=host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable",
		"-k=secret",
		"--audit-file=audit.log",
		"--audit-url=https://audit.com",
		"--crypto-key=server-private.pem",
		"--pprof=localhost:6060",
		"-t=192.168.0.1/24",
	}
	err := os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("GRPC_ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_CERT")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("STORE_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("FILE_STORAGE_PATH")
	require.NoError(t, err)
	err = os.Unsetenv("RESTORE")
	require.NoError(t, err)
	err = os.Unsetenv("DATABASE_DSN")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_FILE")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_URL")
	require.NoError(t, err)
	err = os.Unsetenv("CRYPTO_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("PPROF_ADDR")
	require.NoError(t, err)
	err = os.Unsetenv("TRUSTED_SUBNET")
	require.NoError(t, err)
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.HTTPServerAddr)
	assert.Equal(t, "127.0.0.1:3202", cfg.GRPCServerAddr)
	assert.Equal(t, "server-flag.crt", cfg.TLSCertPath)
	assert.Equal(t, "server-flag.key", cfg.TLSKeyPath)
	assert.Equal(t, convertIntToConfigDuration(200), cfg.StoreInterval)
	assert.Equal(t, "metrics.json", cfg.FileStoragePath)
	assert.Equal(t, true, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable", cfg.DatabaseDSN)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, "audit.log", cfg.AuditFile)
	assert.Equal(t, "https://audit.com", cfg.AuditURL)
	assert.Equal(t, "server-private.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "localhost:6060", cfg.PprofAddr)
	assert.Equal(t, "192.168.0.1/24", cfg.TrustedSubnet)
}

// TestCanGetServerConfig_Env проверяет парсинг переменных окружения сервера.
func TestCanGetServerConfig_Env(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"server"}
	err := os.Setenv("ADDRESS", "localhost:8085")
	require.NoError(t, err)
	err = os.Setenv("GRPC_ADDRESS", "localhost:3203")
	require.NoError(t, err)
	err = os.Setenv("TLS_CERT", "server-env.crt")
	require.NoError(t, err)
	err = os.Setenv("TLS_KEY", "server-env.key")
	require.NoError(t, err)
	err = os.Setenv("STORE_INTERVAL", "250")
	require.NoError(t, err)
	err = os.Setenv("FILE_STORAGE_PATH", "my_metric.json")
	require.NoError(t, err)
	err = os.Setenv("RESTORE", "false")
	require.NoError(t, err)
	err = os.Setenv("DATABASE_DSN", "host=host port=port user=myuser password=xxxx dbname=mydb sslmode=disable")
	require.NoError(t, err)
	err = os.Setenv("KEY", "everybodyknows")
	require.NoError(t, err)
	err = os.Setenv("AUDIT_FILE", "audit.log")
	require.NoError(t, err)
	err = os.Setenv("AUDIT_URL", "https://audit.com")
	require.NoError(t, err)
	err = os.Setenv("CRYPTO_KEY", "server-env-private.pem")
	require.NoError(t, err)
	err = os.Setenv("PPROF_ADDR", "localhost:6060")
	require.NoError(t, err)
	err = os.Setenv("TRUSTED_SUBNET", "192.168.0.1/24")
	require.NoError(t, err)
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.HTTPServerAddr)
	assert.Equal(t, "localhost:3203", cfg.GRPCServerAddr)
	assert.Equal(t, "server-env.crt", cfg.TLSCertPath)
	assert.Equal(t, "server-env.key", cfg.TLSKeyPath)
	assert.Equal(t, convertIntToConfigDuration(250), cfg.StoreInterval)
	assert.Equal(t, "my_metric.json", cfg.FileStoragePath)
	assert.Equal(t, false, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=xxxx dbname=mydb sslmode=disable", cfg.DatabaseDSN)
	assert.Equal(t, "everybodyknows", cfg.SignatureKey)
	assert.Equal(t, "audit.log", cfg.AuditFile)
	assert.Equal(t, "https://audit.com", cfg.AuditURL)
	assert.Equal(t, "server-env-private.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "localhost:6060", cfg.PprofAddr)
	assert.Equal(t, "192.168.0.1/24", cfg.TrustedSubnet)
}

// TestCanGetServerConfig_FlagOverJSONPrecedence проверяет приоритет параметров командной строки сервера над файлом
// конфигурации.
func TestCanGetServerConfig_FlagOverJSONPrecedence(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	configPath := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(configPath, []byte(`
{
  "address":"localhost:9090",
  "grpc_address":"localhost:3202",
  "tls_cert": "server-json.crt",
  "tls_key": "server-json.key",
  "store_interval":"300s",
  "store_file":"server-metrics.json",
  "restore":false,
  "database_dsn":"postgres://metrics",
  "signature_key": "secret",
  "audit_file": "auditA.json",
  "audit_url": "https://audit.site/hook",
  "crypto_key":"server-private.pem",
  "pprof_address": "localhost:6060",
  "trusted_subnet": "192.168.0.1/24"
}
`), 0o600)
	require.NoError(t, err)

	os.Args = []string{
		"server",
		"-a=127.0.0.1:8081",
		"--grpc-address=127.0.0.1:3204",
		"--tls-cert=server-flag.crt",
		"--tls-key=server-flag.key",
		"-i=200",
		"-f=metrics.json",
		"-r",
		"-d=host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable",
		"-k=secret",
		"--audit-file=auditB.log",
		"--audit-url=https://audit.com",
		"--crypto-key=server-flag-private.pem",
		"--pprof=localhost:6061",
		"-t=192.168.0.1/16",
		"-c=" + configPath,
	}
	err = os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("GRPC_ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_CERT")
	require.NoError(t, err)
	err = os.Unsetenv("TLS_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("STORE_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("FILE_STORAGE_PATH")
	require.NoError(t, err)
	err = os.Unsetenv("RESTORE")
	require.NoError(t, err)
	err = os.Unsetenv("DATABASE_DSN")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_FILE")
	require.NoError(t, err)
	err = os.Unsetenv("AUDIT_URL")
	require.NoError(t, err)
	err = os.Unsetenv("CRYPTO_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("PPROF_ADDR")
	require.NoError(t, err)
	err = os.Unsetenv("TRUSTED_SUBNET")
	require.NoError(t, err)
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.HTTPServerAddr)
	assert.Equal(t, "127.0.0.1:3204", cfg.GRPCServerAddr)
	assert.Equal(t, "server-flag.crt", cfg.TLSCertPath)
	assert.Equal(t, "server-flag.key", cfg.TLSKeyPath)
	assert.Equal(t, convertIntToConfigDuration(200), cfg.StoreInterval)
	assert.Equal(t, "metrics.json", cfg.FileStoragePath)
	assert.Equal(t, true, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable", cfg.DatabaseDSN)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, "auditB.log", cfg.AuditFile)
	assert.Equal(t, "https://audit.com", cfg.AuditURL)
	assert.Equal(t, "server-flag-private.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "localhost:6061", cfg.PprofAddr)
	assert.Equal(t, "192.168.0.1/16", cfg.TrustedSubnet)
}

// TestCanGetServerConfig_EnvOverFlagPrecedence проверяет приоритет переменных окружения над параметрами командной
// строки сервера.
func TestCanGetServerConfig_EnvOverFlagPrecedence(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"server",
		"-a=127.0.0.1:8081",
		"--grpc-address=127.0.0.1:3204",
		"--tls-cert=server-flag.crt",
		"--tls-key=server-flag.key",
		"-i=200",
		"-f=metrics.json",
		"-r",
		"-d=host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable",
		"-k=secret",
		"--audit-file=audit.log",
		"--audit-url=https://audit.com",
		"--crypto-key=server-flag-private.pem",
		"-t=192.168.0.1/16",
		"--pprof=localhost:6060",
	}
	err := os.Setenv("ADDRESS", "localhost:8085")
	require.NoError(t, err)
	err = os.Setenv("GRPC_ADDRESS", "localhost:3205")
	require.NoError(t, err)
	err = os.Setenv("TLS_CERT", "server-env-priority.crt")
	require.NoError(t, err)
	err = os.Setenv("TLS_KEY", "server-env-priority.key")
	require.NoError(t, err)
	err = os.Setenv("STORE_INTERVAL", "250")
	require.NoError(t, err)
	err = os.Setenv("FILE_STORAGE_PATH", "my_metric.json")
	require.NoError(t, err)
	err = os.Setenv("RESTORE", "false")
	require.NoError(t, err)
	err = os.Setenv("DATABASE_DSN", "host=host port=port user=myuser password=xxxx dbname=mydb sslmode=disable")
	require.NoError(t, err)
	err = os.Setenv("KEY", "everybodyknows")
	require.NoError(t, err)
	err = os.Setenv("AUDIT_FILE", "audit-2.log")
	require.NoError(t, err)
	err = os.Setenv("AUDIT_URL", "https://audit-2.com")
	require.NoError(t, err)
	err = os.Setenv("CRYPTO_KEY", "server-env-priority.pem")
	require.NoError(t, err)
	err = os.Setenv("PPROF_ADDR", "localhost:6061")
	require.NoError(t, err)
	err = os.Setenv("TRUSTED_SUBNET", "192.168.0.1/24")
	require.NoError(t, err)
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.HTTPServerAddr)
	assert.Equal(t, "localhost:3205", cfg.GRPCServerAddr)
	assert.Equal(t, "server-env-priority.crt", cfg.TLSCertPath)
	assert.Equal(t, "server-env-priority.key", cfg.TLSKeyPath)
	assert.Equal(t, convertIntToConfigDuration(250), cfg.StoreInterval)
	assert.Equal(t, "my_metric.json", cfg.FileStoragePath)
	assert.Equal(t, false, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=xxxx dbname=mydb sslmode=disable", cfg.DatabaseDSN)
	assert.Equal(t, "everybodyknows", cfg.SignatureKey)
	assert.Equal(t, "audit-2.log", cfg.AuditFile)
	assert.Equal(t, "https://audit-2.com", cfg.AuditURL)
	assert.Equal(t, "server-env-priority.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "localhost:6061", cfg.PprofAddr)
	assert.Equal(t, "192.168.0.1/24", cfg.TrustedSubnet)
}
