package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanGetAgentConfig_Default проверяет поведение по умолчанию, когда ни флаги, ни переменные окружения агента не
// заданы
func TestCanGetAgentConfig_Default(t *testing.T) {
	// Arrange
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"agent"}
	err := os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("REPORT_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("POLL_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, defaultServerAddr, cfg.ServerAddr)
	assert.Equal(t, defaultPollInterval, cfg.PollInterval)
	assert.Equal(t, defaultReportInterval, cfg.ReportInterval)
	assert.Equal(t, "", cfg.Key)
}

// TestCanGetAgentConfig_Flag проверяет парсинг параметров командной строки агента
func TestCanGetAgentConfig_Flag(t *testing.T) {
	// Arrange
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"agent", "-a=127.0.0.1:8081", "-r=5", "-p=1", "-k=secret"}
	err := os.Unsetenv("ADDRESS")
	require.NoError(t, err)
	err = os.Unsetenv("REPORT_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("POLL_INTERVAL")
	require.NoError(t, err)
	err = os.Unsetenv("KEY")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.ServerAddr)
	assert.Equal(t, 5, cfg.ReportInterval)
	assert.Equal(t, 1, cfg.PollInterval)
	assert.Equal(t, "secret", cfg.Key)
}

// TestCanGetAgentConfig_Env проверяет парсинг переменных окружения агента
func TestCanGetAgentConfig_Env(t *testing.T) {
	// Arrange
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"agent"}
	err := os.Setenv("ADDRESS", "127.0.0.1:8081")
	require.NoError(t, err)
	err = os.Setenv("REPORT_INTERVAL", "5")
	require.NoError(t, err)
	err = os.Setenv("POLL_INTERVAL", "1")
	require.NoError(t, err)
	err = os.Setenv("KEY", "everybodyknows")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", cfg.ServerAddr)
	assert.Equal(t, 5, cfg.ReportInterval)
	assert.Equal(t, 1, cfg.PollInterval)
	assert.Equal(t, "everybodyknows", cfg.Key)
}

// TestCanGetAgentConfig_EnvPrecedence проверяет приоритет переменных окружения над параметрами командной строки агента
func TestCanGetAgentConfig_EnvPrecedence(t *testing.T) {
	// Arrange
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"agent", "-a=127.0.0.1:8081", "-r=5", "-p=1", "-k=secret"}
	err := os.Setenv("ADDRESS", "localhost:8085")
	require.NoError(t, err)
	err = os.Setenv("REPORT_INTERVAL", "15")
	require.NoError(t, err)
	err = os.Setenv("POLL_INTERVAL", "3")
	require.NoError(t, err)
	err = os.Setenv("KEY", "everybodyknows")
	require.NoError(t, err)

	// Act
	cfg, err := GetAgentConfig()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "localhost:8085", cfg.ServerAddr)
	assert.Equal(t, 15, cfg.ReportInterval)
	assert.Equal(t, 3, cfg.PollInterval)
	assert.Equal(t, "everybodyknows", cfg.Key)
}
