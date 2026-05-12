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

// TestCanGetAgentConfig_Default проверяет поведение по умолчанию, когда ни файл конфигурации, ни флаги, ни
// переменные окружения агента не заданы.
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
	assert.Equal(t, convertIntToConfigDuration(defaultPollInterval), cfg.PollInterval)
	assert.Equal(t, convertIntToConfigDuration(defaultReportInterval), cfg.ReportInterval)
	assert.Equal(t, "", cfg.SignatureKey)
	assert.Equal(t, defaultRateLimit, cfg.RateLimit)
	assert.Equal(t, "", cfg.CryptoKeyPath)
}

// TestCanGetAgentConfig_JSON_FromFlag проверяет парсинг файла конфигурации, имя которого было передано через флаг.
func TestCanGetAgentConfig_JSON_FromFlag(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	configPath := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(configPath, []byte(`
{
  "address":"localhost:9090",
  "report_interval":"15s",
  "poll_interval":"5s",
  "signature_key":"secret",
  "rate_limit":5,  
  "crypto_key":"agent-public.pem"
}
`), 0o600)
	require.NoError(t, err)
	os.Args = []string{"agent", "-c", configPath}

	err = os.Unsetenv("ADDRESS")
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
	assert.Equal(t, "localhost:9090", cfg.ServerAddr)
	assert.Equal(t, configDuration(15*time.Second), cfg.ReportInterval)
	assert.Equal(t, configDuration(5*time.Second), cfg.PollInterval)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, 5, cfg.RateLimit)
	assert.Equal(t, "agent-public.pem", cfg.CryptoKeyPath)
}

// TestCanGetAgentConfig_JSON_FromEnv проверяет парсинг файла конфигурации, имя которого было передано через переменную
// окружения.
func TestCanGetAgentConfig_JSON_FromEnv(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	configPath := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(configPath, []byte(`
{
  "address":"localhost:9090",
  "report_interval":"15s",
  "poll_interval":"5s",
  "signature_key":"secret",
  "rate_limit":5,  
  "crypto_key":"agent-public.pem"
}
`), 0o600)
	require.NoError(t, err)

	err = os.Unsetenv("ADDRESS")
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
	err = os.Setenv("CONFIG", configPath)
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:9090", cfg.ServerAddr)
	assert.Equal(t, configDuration(15*time.Second), cfg.ReportInterval)
	assert.Equal(t, configDuration(5*time.Second), cfg.PollInterval)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, 5, cfg.RateLimit)
	assert.Equal(t, "agent-public.pem", cfg.CryptoKeyPath)
}

// TestCanGetAgentConfig_Flag проверяет парсинг параметров командной строки агента.
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
	assert.Equal(t, convertIntToConfigDuration(5), cfg.ReportInterval)
	assert.Equal(t, convertIntToConfigDuration(1), cfg.PollInterval)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, 15, cfg.RateLimit)
	assert.Equal(t, "agent-public.pem", cfg.CryptoKeyPath)
}

// TestCanGetAgentConfig_Env проверяет парсинг переменных окружения агента.
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
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.ServerAddr)
	assert.Equal(t, convertIntToConfigDuration(5), cfg.ReportInterval)
	assert.Equal(t, convertIntToConfigDuration(1), cfg.PollInterval)
	assert.Equal(t, "everybodyknows", cfg.SignatureKey)
	assert.Equal(t, 20, cfg.RateLimit)
	assert.Equal(t, "agent-env-public.pem", cfg.CryptoKeyPath)
}

// TestCanGetAgentConfig_FlagOverJSONPrecedence проверяет приоритет параметров командной строки агента над файлом
// конфигурации.
func TestCanGetAgentConfig_FlagOverJSONPrecedence(t *testing.T) {
	// Arrange
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	configPath := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(configPath, []byte(`
{
  "address":"localhost:9090",
  "report_interval":"15s",
  "poll_interval":"5s",
  "signature_key":"everybodyknows",
  "rate_limit":5,  
  "crypto_key":"agent-public.pem"
}
`), 0o600)
	require.NoError(t, err)

	os.Args = []string{
		"agent",
		"-a=127.0.0.1:8081",
		"-r=5",
		"-p=1",
		"-k=secret",
		"--crypto-key=agent-flag-public.pem",
		"-l=15",
		"-c=" + configPath,
	}
	err = os.Unsetenv("ADDRESS")
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
	assert.Equal(t, convertIntToConfigDuration(5), cfg.ReportInterval)
	assert.Equal(t, convertIntToConfigDuration(1), cfg.PollInterval)
	assert.Equal(t, "secret", cfg.SignatureKey)
	assert.Equal(t, "agent-flag-public.pem", cfg.CryptoKeyPath)
	assert.Equal(t, 15, cfg.RateLimit)
}

// TestCanGetAgentConfig_EnvOverFlagPrecedence проверяет приоритет переменных окружения над параметрами командной
// строки агента.
func TestCanGetAgentConfig_EnvOverFlagPrecedence(t *testing.T) {
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
	err = os.Unsetenv("CONFIG")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.ServerAddr)
	assert.Equal(t, convertIntToConfigDuration(15), cfg.ReportInterval)
	assert.Equal(t, convertIntToConfigDuration(3), cfg.PollInterval)
	assert.Equal(t, "everybodyknows", cfg.SignatureKey)
	assert.Equal(t, "agent-env-priority.pem", cfg.CryptoKeyPath)
	assert.Equal(t, 20, cfg.RateLimit)
}
