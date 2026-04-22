package osexitmain

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

// TestOsExitMainAnalyzer запускает Analyzer на пакетах из ./testdata и проверяет ожидания, заданные через комментарии
// вида "// want ...".
func TestOsExitMainAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "./...")
}
