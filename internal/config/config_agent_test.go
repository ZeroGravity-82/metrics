package config

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanGetAgentConfig_Default проверяет поведение по умолчанию, когда ни флаги, ни переменные окружения агента не
// заданы
func TestCanGetAgentConfig_Default(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"agent"}
	err := os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("REPORT_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("POLL_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)
	err = os.Unsetenv("RATE_LIMIT")
	require.NoError(t, err)
	err = os.Unsetenv("CRYPTO_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, defaultServerAddr, cfg.ServerAddr)
	assert.Equal(t, defaultPollInterval, cfg.PollInterval)
	assert.Equal(t, defaultReportInterval, cfg.ReportInterval)
	assert.Equal(t, "", cfg.SignatureKey)
	assert.Equal(t, defaultRateLimit, cfg.RateLimit)
	assert.Equal(t, "", cfg.CryptoKeyPath)
	assert.Equal(t, "", cfg.ConfigFileName)
}

// TestCanGetAgentConfig_Flag проверяет парсинг параметров командной строки агента
func TestCanGetAgentConfig_Flag(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"agent",
		"-a=127.0.0.1:8081",
		"-r=5",
		"-p=1",
		"-k=secret",
		"-l=15",
		"--crypto-key=agent-public.pem",
		"-c=/path/to/config.json",
	}
	err := os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("REPORT_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("POLL_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)
	err = os.Unsetenv("RATE_LIMIT")
	require.NoError(t, err)
	err = os.Unsetenv("CRYPTO_KEY")
	require.NoError(t, err)
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.ServerAddr)
	assert.Equal(t, 5, cfg.ReportInterval)
	assert.Equal(t, 1, cfg.PollInterval)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, 15, cfg.RateLimit)
	assert.Equal(t, "agent-public.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "/path/to/config.json", cfg.ConfigFileName)
}

// TestCanGetAgentConfig_Env проверяет парсинг переменных окружения агента
func TestCanGetAgentConfig_Env(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{"agent"}
	err := os.Setenv("ADDRESS", "127.0.0.1:8081")
	require.NoError(t, err)
	err = os.Setenv("REPORT_INTERVAL", "5")
	require.NoError(t, err)
	err = os.Setenv("POLL_INTERVAL", "1")
	require.NoError(t, err)
	err = os.Setenv("KEY", "everybodyknows")
	require.NoError(t, err)
	err = os.Setenv("RATE_LIMIT", "20")
	require.NoError(t, err)
	err = os.Setenv("CRYPTO_KEY", "agent-env-public.pem")
	require.NoError(t, err)
	err = os.Setenv("CONFIG", "/path/to/config.json")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.ServerAddr)
	assert.Equal(t, 5, cfg.ReportInterval)
	assert.Equal(t, 1, cfg.PollInterval)
	assert.Equal(t, "everybodyknows", cfg.SignatureKey)
	assert.Equal(t, 20, cfg.RateLimit)
	assert.Equal(t, "agent-env-public.pem", cfg.CryptoKeyPath)
	assert.Equal(t, "/path/to/config.json", cfg.ConfigFileName)
}

// TestCanGetAgentConfig_EnvPrecedence проверяет приоритет переменных окружения над параметрами командной строки агента
func TestCanGetAgentConfig_EnvPrecedence(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = []string{
		"agent",
		"-a=127.0.0.1:8081",
		"-r=5",
		"-p=1",
		"-k=secret",
		"--crypto-key=agent-flag-public.pem",
		"-l=15",
		"-c=/path/to/configA.json",
	}
	err := os.Setenv("ADDRESS", "localhost:8085")
	require.NoError(t, err)
	err = os.Setenv("REPORT_INTERVAL", "15")
	require.NoError(t, err)
	err = os.Setenv("POLL_INTERVAL", "3")
	require.NoError(t, err)
	err = os.Setenv("KEY", "everybodyknows")
	require.NoError(t, err)
	err = os.Setenv("RATE_LIMIT", "20")
	require.NoError(t, err)
	err = os.Setenv("CRYPTO_KEY", "agent-env-priority.pem")
	require.NoError(t, err)
	err = os.Setenv("CONFIG", "/path/to/configB.json")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.ServerAddr)
	assert.Equal(t, 15, cfg.ReportInterval)
	assert.Equal(t, 3, cfg.PollInterval)
	assert.Equal(t, "everybodyknows", cfg.SignatureKey)
	assert.Equal(t, "agent-env-priority.pem", cfg.CryptoKeyPath)
	assert.Equal(t, 20, cfg.RateLimit)
	assert.Equal(t, "/path/to/configB.json", cfg.ConfigFileName)
}
