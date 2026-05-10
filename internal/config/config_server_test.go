package config

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanGetServerConfig_Default проверяет поведение по умолчанию, когда ни флаги, ни переменные окружения сервера не
// заданы
func TestCanGetServerConfig_Default(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"server"}
	err := os.Unsetenv("ADDRESS")
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
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, defaultServerAddr, cfg.ServerAddr)
	assert.Equal(t, defaultStoreInterval, cfg.StoreInterval)
	assert.Equal(t, "", cfg.FileStoragePath)
	assert.Equal(t, defaultRestore, cfg.Restore)
	assert.Equal(t, "", cfg.DatabaseDSN)
	assert.Equal(t, "", cfg.SignatureKey)
	assert.Equal(t, "", cfg.AuditFile)
	assert.Equal(t, "", cfg.AuditURL)
	assert.Equal(t, "", cfg.CryptoKeyPath)
	assert.Equal(t, "", cfg.ConfigFileName)
}

// TestCanGetServerConfig_Flag проверяет парсинг параметров командной строки сервера
func TestCanGetServerConfig_Flag(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"server",
		"-a=127.0.0.1:8081",
		"-i=200",
		"-f=metrics.json",
		"-r",
		"-d=host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable",
		"-k=secret",
		"--audit-file=audit.log",
		"--audit-url=https://audit.com",
		"--crypto-key=server-private.pem",
		"-c=/path/to/config.json",
	}
	err := os.Unsetenv("ADDRESS")
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
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.ServerAddr)
	assert.Equal(t, 200, cfg.StoreInterval)
	assert.Equal(t, "metrics.json", cfg.FileStoragePath)
	assert.Equal(t, true, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable", cfg.DatabaseDSN)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, "audit.log", cfg.AuditFile)
	assert.Equal(t, "https://audit.com", cfg.AuditURL)
	assert.Equal(t, "server-private.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "/path/to/config.json", cfg.ConfigFileName)
}

// TestCanGetServerConfig_Env проверяет парсинг переменных окружения сервера
func TestCanGetServerConfig_Env(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"server"}
	err := os.Setenv("ADDRESS", "localhost:8085")
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
	err = os.Setenv("CONFIG", "/path/to/config.json")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.ServerAddr)
	assert.Equal(t, 250, cfg.StoreInterval)
	assert.Equal(t, "my_metric.json", cfg.FileStoragePath)
	assert.Equal(t, false, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=xxxx dbname=mydb sslmode=disable", cfg.DatabaseDSN)
	assert.Equal(t, "everybodyknows", cfg.SignatureKey)
	assert.Equal(t, "audit.log", cfg.AuditFile)
	assert.Equal(t, "https://audit.com", cfg.AuditURL)
	assert.Equal(t, "server-env-private.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "/path/to/config.json", cfg.ConfigFileName)
}

// TestCanGetServerConfig_EnvPrecedence проверяет приоритет переменных окружения над параметрами командной строки
// сервера
func TestCanGetServerConfig_EnvPrecedence(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"server",
		"-a=127.0.0.1:8081",
		"-i=200",
		"-f=metrics.json",
		"-r",
		"-d=host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable",
		"-k=secret",
		"--audit-file=audit.log",
		"--audit-url=https://audit.com",
		"--crypto-key=server-flag-private.pem",
		"-c=/path/to/configA.json",
	}
	err := os.Setenv("ADDRESS", "localhost:8085")
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
	err = os.Setenv("CONFIG", "/path/to/configB.json")
	require.NoError(t, err)

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.ServerAddr)
	assert.Equal(t, 250, cfg.StoreInterval)
	assert.Equal(t, "my_metric.json", cfg.FileStoragePath)
	assert.Equal(t, false, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=xxxx dbname=mydb sslmode=disable", cfg.DatabaseDSN)
	assert.Equal(t, "everybodyknows", cfg.SignatureKey)
	assert.Equal(t, "audit-2.log", cfg.AuditFile)
	assert.Equal(t, "https://audit-2.com", cfg.AuditURL)
	assert.Equal(t, "server-env-priority.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "/path/to/configB.json", cfg.ConfigFileName)
}
