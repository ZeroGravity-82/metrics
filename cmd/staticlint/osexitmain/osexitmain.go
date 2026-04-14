// Пакет osexitmain содержит собственный анализатор для golang.org/x/tools/go/analysis.
//
// Анализатор находит прямые вызовы os.Exit() внутри func main() пакета main.
//
// Что считается нарушением:
//   - os.Exit(1).
//   - alias.Exit(1) (import alias "os").
//   - Exit(1) (dot-import: import . "os").
//
// Что НЕ считается нарушением:
//   - любые вызовы os.Exit() вне пакета main.
//   - вызовы os.Exit() вне функции main(), включая вызовы из вложенных функций.
package osexitmain

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer - экспортируемая точка входа, через которую multichecker подключает этот анализатор.
var Analyzer = &analysis.Analyzer{
	Name: "osexitmain",
	Doc:  "reports os.Exit calls inside main() of package main",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Нас интересует только пакет main.
	if pass.Pkg == nil || pass.Pkg.Name() != "main" {
		return nil, nil
	}
	// Игнорируем сгенерированные тестовые main-пакеты. Они лежат в ~/.cache/go-build и содержат os.Exit() по
	// определению. Их анализ не несет пользы и только засоряет вывод линтера.
	if strings.HasSuffix(pass.Pkg.Path(), ".test") {
		return nil, nil
	}

	// Идём сверху-вниз: находим именно func main() (без ресивера).
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Name == nil || fd.Name.Name != "main" || fd.Body == nil {
				continue
			}

			// Внутри main() ищем вызовы.
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isOsExitCall(pass, call) {
					pass.Reportf(call.Lparen, "os.Exit called inside main()")
				}
				return true
			})
		}
	}
	return nil, nil
}

func isOsExitCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	// Проверка по типам - корректно работает с алиасами импорта, dot-импортами и переименованиями.
	var usedObj types.Object

	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		usedObj = pass.TypesInfo.Uses[fun.Sel]
	case *ast.Ident:
		// Например, при dot-импорте: import . "os"; Exit(1)
		usedObj = pass.TypesInfo.Uses[fun]
	default:
		return false
	}

	fn, ok := usedObj.(*types.Func)
	if !ok {
		return false
	}

	pkg := fn.Pkg()
	return pkg != nil && pkg.Path() == "os" && fn.Name() == "Exit"
}
