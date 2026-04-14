package osexitmain

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "osexitmain",
	Doc:  `reports os.Exit calls inside main() of package main`,
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	// Нас интересует только пакет main.
	if pass.Pkg == nil || pass.Pkg.Name() != "main" {
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
