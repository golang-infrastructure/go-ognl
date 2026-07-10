package ognl_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestIssue33PublicAPISurfaceSnapshot(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseDir(fset, ".", func(info os.FileInfo) bool {
		return strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	syntaxPackage, ok := parsed["ognl"]
	if !ok {
		t.Fatal("package ognl was not found")
	}
	files := make([]*ast.File, 0, len(syntaxPackage.Files))
	for _, file := range syntaxPackage.Files {
		files = append(files, file)
	}
	checked, err := (&types.Config{Importer: importer.Default()}).Check("github.com/golang-infrastructure/go-ognl", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}

	got := issue33ExportedAPI(checked)
	want := []string{
		"const Array Type",
		"const Bool Type",
		"const Chan Type",
		"const Complex128 Type",
		"const Complex64 Type",
		"const Float32 Type",
		"const Float64 Type",
		"const Func Type",
		"const Int Type",
		"const Int16 Type",
		"const Int32 Type",
		"const Int64 Type",
		"const Int8 Type",
		"const Interface Type",
		"const Invalid Type",
		"const Map Type",
		"const Pointer Type",
		"const Slice Type",
		"const String Type",
		"const Struct Type",
		"const Uint Type",
		"const Uint16 Type",
		"const Uint32 Type",
		"const Uint64 Type",
		"const Uint8 Type",
		"const Uintptr Type",
		"const UnsafePointer Type",
		"func Get func(value interface{}, path string) Result",
		"func GetE func(value interface{}, path string) (Result, error)",
		"func GetMany func(value interface{}, path ...string) []Result",
		"func Parse func(result interface{}) Result",
		"method Result.Diagnosis func() []error",
		"method Result.Effective func() bool",
		"method Result.Get func(path string) Result",
		"method Result.GetE func(path string) (Result, error)",
		"method Result.Type func() Type",
		"method Result.Value func() interface{}",
		"method Result.Values func() []interface{}",
		"method Result.ValuesE func() ([]interface{}, error)",
		"method Type.String func() string",
		"type Result Result",
		"type Type Type",
		"var ErrIndexOutOfBounds error",
		"var ErrInvalidStructure error",
		"var ErrInvalidValue error",
		"var ErrMapKeyMustInt error",
		"var ErrMapKeyMustString error",
		"var ErrParseInt error",
		"var ErrSliceSubscript error",
		"var ErrStructIndexOutOfBounds error",
		"var ErrUnableExpand error",
	}
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("public API changed:\n--- got ---\n%s\n--- want ---\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func issue33ExportedAPI(pkg *types.Package) []string {
	qualifier := func(other *types.Package) string {
		if other == pkg {
			return ""
		}
		return other.Name()
	}
	var api []string
	for _, name := range pkg.Scope().Names() {
		object := pkg.Scope().Lookup(name)
		if !object.Exported() {
			continue
		}
		kind := ""
		switch object.(type) {
		case *types.Const:
			kind = "const"
		case *types.Func:
			kind = "func"
		case *types.TypeName:
			kind = "type"
		case *types.Var:
			kind = "var"
		default:
			continue
		}
		api = append(api, kind+" "+name+" "+types.TypeString(object.Type(), qualifier))

		typeName, ok := object.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			continue
		}
		for i := 0; i < named.NumMethods(); i++ {
			method := named.Method(i)
			if method.Exported() {
				api = append(api, "method "+name+"."+method.Name()+" "+types.TypeString(method.Type(), qualifier))
			}
		}
	}
	sort.Strings(api)
	return api
}
