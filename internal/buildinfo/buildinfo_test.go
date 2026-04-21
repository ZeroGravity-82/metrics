package buildinfo

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrint проверяет вывод метаданных сборки для пустых и заполненных значений.
func TestPrint(t *testing.T) {
	// Arrange
	tests := []struct {
		name         string
		buildVersion string
		buildDate    string
		buildCommit  string
		wantOutput   string
	}{
		{
			name:       "prints defaults for empty values",
			wantOutput: "Build version: N/A\nBuild date: N/A\nBuild commit: N/A\n",
		},
		{
			name:         "prints provided values",
			buildVersion: "1.2.3",
			buildDate:    "2026-04-21T14:43:30Z",
			buildCommit:  "440520f",
			wantOutput:   "Build version: 1.2.3\nBuild date: 2026-04-21T14:43:30Z\nBuild commit: 440520f\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			output := captureStdout(t, func() {
				Print(tt.buildVersion, tt.buildDate, tt.buildCommit)
			})

			// Assert
			assert.Equal(t, tt.wantOutput, output)
		})
	}
}

// captureStdout временно перенаправляет stdout в pipe и возвращает весь записанный вывод.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper() // нужен, чтобы место ошибки require.NoError отображалось в TestPrint, а не здесь

	// Arrange
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	// Act
	fn()
	defer func() {
		_ = reader.Close()
	}()
	err = writer.Close() // закрываем writer, чтобы следом можно было начать читать вывод из pipe
	require.NoError(t, err)

	// Assert
	output, err := io.ReadAll(reader)
	require.NoError(t, err)

	return string(output)
}
