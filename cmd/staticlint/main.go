// Команда staticlint - это multichecker, собранный поверх golang.org/x/tools/go/analysis/multichecker.
//
// # Как это работает
//
// multichecker принимает список пакетов/паттернов и запускает по ним каждый анализатор. Далее точка входа вызывает
// multichecker.Main и передает туда список анализаторов, формируемый в функции analyzers().
//
// # Запуск
//
//	go run ./cmd/staticlint ./...
//
// Примеры паттернов:
//   - ./... (все пакеты)
//   - ./cmd/... (только команды)
//   - ./internal/... (только внутренние пакеты)
//
// # Включенные анализаторы
//
// Данный multichecker состоит из:
//
//  1. Набора стандартных анализаторов из golang.org/x/tools/go/analysis/passes:
//     - printf.Analyzer: проверяет корректность строк с форматированием и их аргументов в Printf-подобных вызовах.
//     - shadow.Analyzer: находит потенциально ошибочное затенение переменных.
//     - structtag.Analyzer: валидирует теги полей структур (синтаксис, дубликаты и т.п.).
//
//  2. Анализаторов из пакета honnef.co/go/tools/staticcheck:
//     - Все анализаторы класса "SA" (класс Staticcheck - ошибки/подозрительные места в коде).
//     - По одному анализатору остальных классов пакета:
//     - S1012: предлагает заменить time.Now().Sub(x) на time.Since(x).
//     - ST1005: проверяет форматирование строк ошибок (error string style).
//     - QF1011: предлагает убрать избыточный тип в объявлении переменной.
//
//  3. Двух дополнительных публичных анализаторов:
//     - gocritic.Analyzer (github.com/go-critic/go-critic): набор стилистических проверок и проверок на потенциальные
//     ошибки.
//     - gosec.Analyzer (github.com/securego/gosec/v2/goanalysis): проверки безопасности (опасные паттерны,
//     крипто-ошибки и т.п.).
//
//  4. Собственного анализатора:
//     - osexitmain.Analyzer: запрещает прямой вызов os.Exit() внутри main() пакета main.
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
		"S1012":  {},
		"ST1005": {},
		"QF1011": {},
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
