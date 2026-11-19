package dialguard_test

import (
	"context"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/opnsense"
	"github.com/ginsys/shelly-manager/internal/shelly"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
	"github.com/ginsys/shelly-manager/internal/shelly/gen2"
	"github.com/ginsys/shelly-manager/internal/testutil/dialguard"
)

func TestMain(m *testing.M) { dialguard.Main(m) }

// offHost is TEST-NET-1 (RFC 5737). The guard refuses it before any packet is sent. The tests
// that use it are only safe to run where nothing can leave the host: an empty network namespace
// is the way to prove it (see docs/testing/testing-strategy.md).
const offHost = "192.0.2.1"

// Each client kind the guard covers must be refused on its own: a guard that works for
// http.DefaultClient says nothing about a client that builds its own transport.
func TestGuardRefusesEachClientKind(t *testing.T) {
	op, err := opnsense.NewClient(opnsense.ClientConfig{Host: offHost, Port: 80, APIKey: "k", APISecret: "s", Timeout: time.Second}, logging.GetDefault())
	if err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func(ctx context.Context) error{
		"http.DefaultClient": func(ctx context.Context) error {
			req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+offHost+"/", nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
			return err
		},
		"gen1":     func(ctx context.Context) error { _, err := gen1.NewClient(offHost).GetInfo(ctx); return err },
		"gen2":     func(ctx context.Context) error { _, err := gen2.NewClient(offHost).GetInfo(ctx); return err },
		"opnsense": func(ctx context.Context) error { return op.TestConnection(ctx) },
		"shelly.DetectGeneration": func(ctx context.Context) error {
			_, err := shelly.DetectGeneration(ctx, offHost)
			return err
		},
		"net.LookupIP": func(context.Context) error { _, err := net.LookupIP("device.example.invalid"); return err },
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		got := dialguard.Capture()
		err := call(ctx)
		cancel()
		refused := got()
		if err == nil || len(refused) == 0 || !strings.Contains(strings.Join(refused, "\n"), "dialguard") {
			t.Errorf("%s: err %v, refused %v", name, err, refused)
		}
	}
}

func TestGuardNamesTheAddress(t *testing.T) {
	got := dialguard.Capture()
	_, err := dialguard.Dial(context.Background(), "tcp", offHost+":80")
	refused := got()
	if err == nil || len(refused) != 1 || !strings.Contains(refused[0], offHost+":80") || !strings.Contains(err.Error(), offHost+":80") {
		t.Fatalf("err %v, refused %v", err, refused)
	}
}

// A loopback address with an IPv6 zone is still loopback. The dial itself may fail (nothing
// listens, or the host has no IPv6): the test is that the guard does not refuse it.
func TestGuardAllowsScopedIPv6Loopback(t *testing.T) {
	got := dialguard.Capture()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if conn, err := dialguard.Dial(ctx, "tcp6", "[::1%lo]:1"); err == nil {
		_ = conn.Close()
	}
	if r := got(); len(r) != 0 {
		t.Fatalf("scoped loopback refused: %v", r)
	}
}

func TestGuardAllowsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer srv.Close()
	got := dialguard.Capture()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	// A host name that /etc/hosts answers is loopback and needs no DNS: the replaced resolver
	// keeps PreferGo, which reads the hosts file before it dials a server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	conn, err := (&net.Dialer{}).DialContext(context.Background(), "tcp", "localhost:"+port)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if r := got(); len(r) != 0 {
		t.Fatalf("loopback refused: %v", r)
	}
}

// A package whose tests lack TestMain is unguarded, so the module is checked for it.
func TestEveryTestPackageCallsMain(t *testing.T) {
	missing := packagesWithoutMain(t, filepath.Join("..", "..", ".."))
	if len(missing) != 0 {
		t.Fatalf("test packages with no TestMain that calls dialguard.Main: %v", missing)
	}
}

// The check accepts the call in the internal or the external test package of a directory, and
// finds a directory that has neither.
func TestPackagesWithoutMainReadsBothPackages(t *testing.T) {
	root := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	main := "func TestMain(m *testing.M) { dialguard.Main(m) }\n"
	write("internal/a/a_test.go", "package a\n"+main)
	write("internal/b/b_test.go", "package b_test\n"+main)
	write("internal/b/other_test.go", "package b\n")
	write("internal/c/c_test.go", "package c\nfunc TestC(t *testing.T) {}\n")
	write("internal/d/d_test.go", "package d\nfunc TestMain(m *testing.M) { os.Exit(m.Run()) }\n")
	write("internal/e/e.go", "package e\n")
	write("ui/node_modules/x/x_test.go", "package x\n")
	// The same text in a comment or a string is not a call.
	write("internal/f/f_test.go", "package f\n// "+main+"var s = `"+main+"`\n")
	// A file a build constraint excludes is not a test file of the build.
	write("internal/g/g_test.go", "//go:build ignore\n\npackage g\n"+main)
	write("internal/g/other_test.go", "package g\n")
	got := packagesWithoutMain(t, root)
	if strings.Join(got, ",") != "internal/c,internal/d,internal/f,internal/g" {
		t.Fatalf("got %v", got)
	}
}

// declaresTestMainCallingGuard reports whether the file declares func TestMain whose body calls
// dialguard.Main. It reads the syntax tree, so a comment or a string that holds the same text does
// not count.
func declaresTestMainCallingGuard(path string) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return false, err
	}
	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "TestMain" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Main" {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "dialguard" {
					found = true
				}
			}
			return true
		})
	}
	return found, nil
}

// packagesWithoutMain lists the directories under root that hold a _test.go file and no
// _test.go file that calls dialguard.Main. It skips node_modules and hidden directories.
func packagesWithoutMain(t *testing.T, root string) []string {
	t.Helper()
	hasTests := map[string]bool{}
	hasMain := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == "node_modules" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		dir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		// A file that a build constraint leaves out of the build holds no test at all.
		if ok, matchErr := build.Default.MatchFile(filepath.Dir(path), filepath.Base(path)); matchErr != nil {
			return matchErr
		} else if !ok {
			return nil
		}
		hasTests[dir] = true
		callsMain, parseErr := declaresTestMainCallingGuard(path)
		if parseErr != nil {
			return parseErr
		}
		if callsMain {
			hasMain[dir] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for dir := range hasTests {
		if !hasMain[dir] {
			missing = append(missing, filepath.ToSlash(dir))
		}
	}
	sort.Strings(missing)
	return missing
}
