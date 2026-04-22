package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	marker        = "generate:reset"
	generatedFile = "reset.gen.go"
)

type targetStruct struct {
	name string
	typ  *ast.StructType
}

type pkgInfo struct {
	dir     string
	name    string
	fset    *token.FileSet
	files   []*ast.File
	structs []targetStruct
	info    *types.Info
	pkg     *types.Package
}

// main запускает обход проекта и генерацию итогового файла для пакетов с целевыми структурами, отмеченными маркером.
func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	module, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		log.Fatal(err)
	}

	packages, err := scan(root, module)
	if err != nil {
		log.Fatal(err)
	}

	for _, pkg := range packages {
		if err := writeGenerated(pkg); err != nil {
			log.Fatalf("%s: %v", pkg.dir, err)
		}
	}
}

// modulePath читает путь модуля из go.mod в корне проекта.
func modulePath(goMod string) (string, error) {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in %s", goMod)
}

// scan обходит директории проекта и собирает пакеты, в которых есть целевыми структуры.
func scan(root, module string) ([]pkgInfo, error) {
	var out []pkgInfo
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if skipDir(entry.Name()) {
			return filepath.SkipDir
		}

		pkg, err := loadPackage(root, module, path)
		if err != nil || len(pkg.structs) == 0 {
			return err
		}
		out = append(out, pkg)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(out, func(i, j int) bool { return out[i].dir < out[j].dir })
	return out, nil
}

// skipDir определяет директории, которые не нужно сканировать генератору.
func skipDir(name string) bool {
	switch name {
	case ".git", ".idea", ".vscode", "vendor", "testdata":
		return true
	default:
		return strings.HasPrefix(name, ".")
	}
}

// loadPackage парсит пакет, находит целевые структуры и дополняет их типовой информацией.
func loadPackage(root, module, dir string) (pkgInfo, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return pkgInfo{}, err
	}

	pkg := pkgInfo{dir: dir, fset: fset}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == generatedFile {
			continue
		}

		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return pkgInfo{}, err
		}
		if pkg.name == "" {
			pkg.name = file.Name.Name
		} else if file.Name.Name != pkg.name {
			return pkgInfo{}, fmt.Errorf("multiple packages found in %s", dir)
		}

		pkg.files = append(pkg.files, file)
		pkg.structs = append(pkg.structs, markedStructs(file)...)
	}
	if len(pkg.files) == 0 || len(pkg.structs) == 0 {
		return pkg, nil
	}

	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	cfg := &types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	checked, err := cfg.Check(packagePath(root, module, dir), fset, pkg.files, info)
	if err != nil {
		return pkgInfo{}, err
	}
	pkg.info = info
	pkg.pkg = checked
	return pkg, nil
}

// packagePath строит полный путь импорта пакета относительно корня модуля.
func packagePath(root, module, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return module
	}
	return filepath.ToSlash(filepath.Join(module, rel))
}

// markedStructs возвращает все структуры файла, помеченные маркером generate:reset.
func markedStructs(file *ast.File) []targetStruct {
	var out []targetStruct
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !hasMarker(gen.Doc) && !hasMarker(typeSpec.Doc) {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if ok {
				out = append(out, targetStruct{name: typeSpec.Name.Name, typ: st})
			}
		}
	}
	return out
}

// hasMarker проверяет, содержит ли группа комментариев маркер generate:reset.
func hasMarker(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}
	for _, line := range strings.Split(group.Text(), "\n") {
		if strings.TrimSpace(line) == marker {
			return true
		}
	}
	return false
}

// writeGenerated создает и записывает файл reset.gen.go для одного пакета.
func writeGenerated(pkg pkgInfo) error {
	gen := generator{pkg: pkg, imports: map[string]struct{}{}}
	code, err := gen.file()
	if err != nil {
		return err
	}
	formatted, err := format.Source(code)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pkg.dir, generatedFile), formatted, 0o644)
}

type generator struct {
	pkg     pkgInfo
	imports map[string]struct{}
}

// file собирает содержимое итогового файла с методами Reset для пакета.
func (g *generator) file() ([]byte, error) {
	methods := make([]string, 0, len(g.pkg.structs))
	for _, st := range g.pkg.structs {
		method, err := g.method(st)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", st.name, err)
		}
		methods = append(methods, method)
	}

	var buf bytes.Buffer
	buf.WriteString("// Code generated by reset generator; DO NOT EDIT.\n\n")
	buf.WriteString("package ")
	buf.WriteString(g.pkg.name)
	buf.WriteString("\n")
	if len(g.imports) > 0 {
		buf.WriteString("\nimport (\n")
		for _, path := range sortedKeys(g.imports) {
			buf.WriteString("\t\"")
			buf.WriteString(path)
			buf.WriteString("\"\n")
		}
		buf.WriteString(")\n")
	}
	for _, method := range methods {
		buf.WriteString("\n")
		buf.WriteString(method)
	}
	return buf.Bytes(), nil
}

// method генерирует тело метода Reset для одной структуры.
func (g *generator) method(st targetStruct) (string, error) {
	var buf bytes.Buffer
	receiver := receiverName(st.name)
	buf.WriteString("func (")
	buf.WriteString(receiver)
	buf.WriteString(" *")
	buf.WriteString(st.name)
	buf.WriteString(") Reset() {\n")
	buf.WriteString("\tif ")
	buf.WriteString(receiver)
	buf.WriteString(" == nil {\n\t\treturn\n\t}\n")

	for _, field := range st.typ.Fields.List {
		for _, name := range fieldNames(field) {
			statements, err := g.reset(receiver+"."+name, g.pkg.info.TypeOf(field.Type))
			if err != nil {
				return "", fmt.Errorf("field %s: %w", name, err)
			}
			for _, line := range statements {
				buf.WriteString("\t")
				buf.WriteString(line)
				buf.WriteString("\n")
			}
		}
	}

	buf.WriteString("}\n")
	return buf.String(), nil
}

// fieldNames возвращает имена обычных и встроенных полей структуры.
func fieldNames(field *ast.Field) []string {
	if len(field.Names) > 0 {
		names := make([]string, 0, len(field.Names))
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
		return names
	}
	switch current := field.Type.(type) {
	case *ast.Ident:
		return []string{current.Name}
	case *ast.SelectorExpr:
		return []string{current.Sel.Name}
	case *ast.StarExpr:
		embedded := &ast.Field{Type: current.X}
		return fieldNames(embedded)
	default:
		return nil
	}
}

// receiverName подбирает короткое имя для ресивера в сгенерированном методе.
func receiverName(typeName string) string {
	if typeName == "" {
		return "r"
	}
	name := strings.ToLower(typeName[:1])
	if name == "_" {
		return "r"
	}
	return name
}

// reset строит набор инструкций для сброса значения в зависимости от его типа.
func (g *generator) reset(target string, typ types.Type) ([]string, error) {
	typ = types.Unalias(typ)
	if hasReset(typ) {
		return []string{target + ".Reset()"}, nil
	}

	switch current := typ.(type) {
	case *types.Pointer:
		inner, err := g.reset("(*"+target+")", current.Elem())
		if err != nil || len(inner) == 0 {
			return inner, err
		}
		return []string{fmt.Sprintf("if %s != nil { %s }", target, strings.Join(inner, "; "))}, nil
	case *types.Named:
		if _, ok := current.Underlying().(*types.Slice); ok {
			return []string{fmt.Sprintf("%s = %s[:0]", target, target)}, nil
		}
		if _, ok := current.Underlying().(*types.Map); ok {
			return []string{fmt.Sprintf("clear(%s)", target)}, nil
		}
	case *types.Slice:
		return []string{fmt.Sprintf("%s = %s[:0]", target, target)}, nil
	case *types.Map:
		return []string{fmt.Sprintf("clear(%s)", target)}, nil
	}

	zero, err := g.zero(typ)
	if err != nil {
		return nil, err
	}
	return []string{fmt.Sprintf("%s = %s", target, zero)}, nil
}

// hasReset проверяет, реализует ли тип или его указатель метод Reset без аргументов.
func hasReset(typ types.Type) bool {
	for _, candidate := range []types.Type{typ, types.NewPointer(typ)} {
		methods := types.NewMethodSet(candidate)
		for i := 0; i < methods.Len(); i++ {
			method := methods.At(i).Obj()
			sig, ok := method.Type().(*types.Signature)
			if ok && method.Name() == "Reset" && sig.Params().Len() == 0 && sig.Results().Len() == 0 {
				return true
			}
		}
	}
	return false
}

// zero возвращает выражение с нулевым значением для переданного типа.
func (g *generator) zero(typ types.Type) (string, error) {
	switch current := types.Unalias(typ).(type) {
	case *types.Basic:
		switch {
		case current.Info()&types.IsBoolean != 0:
			return "false", nil
		case current.Info()&(types.IsInteger|types.IsFloat|types.IsComplex) != 0:
			return "0", nil
		case current.Info()&types.IsString != 0:
			return `""`, nil
		default:
			return "nil", nil
		}
	case *types.Pointer, *types.Slice, *types.Map, *types.Interface, *types.Signature, *types.Chan:
		return "nil", nil
	default:
		name, err := g.typeString(typ)
		if err != nil {
			return "", err
		}
		return name + "{}", nil
	}
}

// typeString превращает тип в строку и собирает внешние импорты для генерации.
func (g *generator) typeString(typ types.Type) (string, error) {
	name := types.TypeString(typ, func(other *types.Package) string {
		if other == nil || g.pkg.pkg == nil || other.Path() == g.pkg.pkg.Path() {
			return ""
		}
		g.imports[other.Path()] = struct{}{}
		return other.Name()
	})
	if strings.Contains(name, "invalid type") {
		return "", fmt.Errorf("unsupported type %q", name)
	}
	return name, nil
}

// sortedKeys возвращает отсортированный список строковых ключей мапы.
func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
