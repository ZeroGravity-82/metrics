package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanGetServerConfig_Default проверяет поведение по умолчанию, когда ни флаги, ни переменные окружения сервера не
// заданы
func TestCanGetServerConfig_Default(t *testing.T) {
	// Arrange
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
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

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, defaultServerAddr, cfg.ServerAddr)
	assert.Equal(t, defaultStoreInterval, cfg.StoreInterval)
	assert.Equal(t, "", cfg.FileStoragePath)
	assert.Equal(t, defaultRestore, cfg.Restore)
	assert.Equal(t, "", cfg.DatabaseDSN)
}

// TestCanGetServerConfig_Flag проверяет парсинг параметров командной строки сервера
func TestCanGetServerConfig_Flag(t *testing.T) {
	// Arrange
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{
		"server",
		"-a=127.0.0.1:8081",
		"-i=200",
		"-f=metrics.json",
		"-r",
		"-d=host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable",
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

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.ServerAddr)
	assert.Equal(t, 200, cfg.StoreInterval)
	assert.Equal(t, "metrics.json", cfg.FileStoragePath)
	assert.Equal(t, true, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable", cfg.DatabaseDSN)
}

// TestCanGetServerConfig_Env проверяет парсинг переменных окружения сервера
func TestCanGetServerConfig_Env(t *testing.T) {
	// Arrange
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
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

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.ServerAddr)
	assert.Equal(t, 250, cfg.StoreInterval)
	assert.Equal(t, "my_metric.json", cfg.FileStoragePath)
	assert.Equal(t, false, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=xxxx dbname=mydb sslmode=disable", cfg.DatabaseDSN)
}

// TestCanGetServerConfig_EnvPrecedence проверяет приоритет переменных окружения над параметрами командной строки сервера
func TestCanGetServerConfig_EnvPrecedence(t *testing.T) {
	// Arrange
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{
		"server",
		"-a=127.0.0.1:8081",
		"-i=200",
		"-f=metrics.json",
		"-r",
		"-d=host=host port=port user=myuser password=yyyy dbname=mydb sslmode=disable",
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

	// Act
	cfg, err := GetServerConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.ServerAddr)
	assert.Equal(t, 250, cfg.StoreInterval)
	assert.Equal(t, "my_metric.json", cfg.FileStoragePath)
	assert.Equal(t, false, cfg.Restore)
	assert.Equal(t, "host=host port=port user=myuser password=xxxx dbname=mydb sslmode=disable", cfg.DatabaseDSN)
}
