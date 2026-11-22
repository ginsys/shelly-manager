package version

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const modulePath = "github.com/ginsys/shelly-manager/"

var (
	argVersion = regexp.MustCompile(`(?m)^ARG VERSION=(\S+)\s*$`)
	stampFlag  = regexp.MustCompile(`-X ([\w./-]+)\.(\w+)=\$\{VERSION\}`)
)

// TestDockerfilesStampTheVersionVariable keeps both images stamping the variable this package
// declares. The Go linker ignores -X for a name that does not exist, so a moved or renamed
// variable would leave every image reporting "dev" with no error anywhere.
func TestDockerfilesStampTheVersionVariable(t *testing.T) {
	for _, name := range []string{"Dockerfile.manager", "Dockerfile.provisioner"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "docker", name))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)

			arg := argVersion.FindStringSubmatch(text)
			if arg == nil {
				t.Fatalf("%s has no line `ARG VERSION=<default>`", name)
			}
			if arg[1] != "dev" {
				t.Errorf("%s: ARG VERSION defaults to %q, want %q", name, arg[1], "dev")
			}

			flag := stampFlag.FindStringSubmatch(text)
			if flag == nil {
				t.Fatalf("%s has no `-X <package>.<variable>=${VERSION}` in its ldflags", name)
			}
			pkg, variable := flag[1], flag[2]
			dir, ok := strings.CutPrefix(pkg, modulePath)
			if !ok {
				t.Fatalf("%s: -X names package %q, outside module %q", name, pkg, modulePath)
			}
			if !declaresVar(t, filepath.Join("..", "..", filepath.FromSlash(dir)), variable) {
				t.Errorf("%s: -X names %s.%s, but %s declares no top-level var %s", name, pkg, variable, dir, variable)
			}
		})
	}
}

// declaresVar reports whether a non-test Go file in dir declares a top-level var of that name.
func declaresVar(t *testing.T, dir, name string) bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				for _, id := range spec.(*ast.ValueSpec).Names {
					if id.Name == name {
						return true
					}
				}
			}
		}
	}
	return false
}
