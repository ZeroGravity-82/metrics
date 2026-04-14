package osexitmain

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestOsExitMainAnalyzer(t *testing.T) {
	// Функция analysistest.Run применяет тестируемый анализатор osexitmain,Analyzer к пакетам из папки testdata
	// и проверяет ожидания.
	// ./... — проверка всех поддиректорий в testdata.
	analysistest.Run(t, analysistest.TestData(), Analyzer, "./...")
}
