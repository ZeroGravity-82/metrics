package main

import (
	"strings"

	gocritic "github.com/go-critic/go-critic/checkers/analyzer"
	gosec "github.com/securego/gosec/v2/goanalysis"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"honnef.co/go/tools/staticcheck"

	"zerogravity-82/metrics/cmd/staticlint/osexitmain"
)

func main() {
	multichecker.Main(analyzers()...)
}

func analyzers() []*analysis.Analyzer {
	var a []*analysis.Analyzer

	// Добавляем стандартные статические анализаторы пакета golang.org/x/tools/go/analysis/passes.
	a = append(a, printf.Analyzer, shadow.Analyzer, structtag.Analyzer)

	// Добавляем анализаторы из пакета honnef.co/go/tools/staticcheck.
	const mainChecksPrefix = "SA"            // Префикс для основных правил анализаторов.
	additionalChecks := map[string]struct{}{ // Дополнительные правила анализаторов.
		"S1012":  {}, // replace time.Now().Sub(x) with time.Since(x)
		"ST1005": {}, // incorrectly formatted error string
		"QF1011": {}, // omit redundant type from variable declaration
	}
	for _, v := range staticcheck.Analyzers {
		if strings.HasPrefix(v.Analyzer.Name, mainChecksPrefix) {
			a = append(a, v.Analyzer)
			continue
		}
		if _, ok := additionalChecks[v.Analyzer.Name]; ok {
			a = append(a, v.Analyzer)
		}
	}

	// Добавляем публичные анализаторы go-critic и gosec.
	a = append(a, gocritic.Analyzer, gosec.Analyzer)

	// Добавляем собственный анализатор, запрещающий использовать прямой вызов os.Exit() в функции main() пакета main.
	a = append(a, osexitmain.Analyzer)

	return a
}
