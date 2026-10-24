package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gorilla/mux"
)

// Route-contract scanning is compiled only with tests. It walks production UI
// sources, turns each axios call into a (method, template) pair using the
// default base URL from ui/src/api/client.ts, and fails closed on any HTTP
// call it cannot normalize. WebSocket calls are not axios paths; they are
// accepted only when transports.json names that file and line.

const routeContractAPIBase = "/api/v1"

type routeContractCall struct {
	File     string
	Line     int
	Method   string
	Template string
}

type routeContractError struct {
	File string
	Line int
	Msg  string
}

func (e routeContractError) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
}

type routeContractPair struct {
	Method   string
	Template string
}

type routeContractPrec struct {
	Method   string
	Request  string
	Expected string
	Matched  string
}

type routeContractManifest struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Kind     string `json:"kind"`
	Generic  bool   `json:"generic"`
	Method   string `json:"method"`
	Template string `json:"template"`
}

type routeContractBaseline struct {
	Method   string `json:"method"`
	Template string `json:"template"`
	Issue    int    `json:"issue"`
}

type routeContractPrecBaseline struct {
	Method   string `json:"method"`
	Request  string `json:"request"`
	Expected string `json:"expected"`
	Matched  string `json:"matched"`
	Issue    int    `json:"issue"`
}

type routeContractHit struct {
	File    string
	Line    int
	Kind    string
	Arg     string
	Literal string
}

func extractRouteContract(root string) ([]routeContractCall, []routeContractError) {
	files, err := routeContractFiles(root)
	if err != nil {
		return nil, []routeContractError{{File: "ui/src", Line: 1, Msg: err.Error()}}
	}
	manifest, err := loadRouteContractManifest(filepath.Join(root, "internal", "api", "testdata", "routecontract", "transports.json"))
	if err != nil {
		return nil, []routeContractError{{File: "internal/api/testdata/routecontract/transports.json", Line: 1, Msg: err.Error()}}
	}
	return extractRouteContractSources(files, manifest)
}

func extractRouteContractSources(files map[string]string, manifest []routeContractManifest) ([]routeContractCall, []routeContractError) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var calls []routeContractCall
	var hits []routeContractHit
	var errs []routeContractError
	for _, name := range names {
		src := files[name]
		if strings.HasSuffix(name, ".vue") {
			chunks, chunkErrs := vueScriptChunks(name, src)
			errs = append(errs, chunkErrs...)
			for _, chunk := range chunks {
				sc := newRouteScanner(name, chunk.src, chunk.line)
				sc.scanFile()
				calls = append(calls, sc.calls...)
				hits = append(hits, sc.hits...)
				errs = append(errs, sc.errs...)
			}
			continue
		}
		sc := newRouteScanner(name, src, 1)
		sc.scanFile()
		calls = append(calls, sc.calls...)
		hits = append(hits, sc.hits...)
		errs = append(errs, sc.errs...)
	}
	applied, manifestErrs := applyRouteContractManifest(calls, hits, manifest, files)
	errs = append(errs, manifestErrs...)
	sort.Slice(applied, func(i, j int) bool {
		if applied[i].File != applied[j].File {
			return applied[i].File < applied[j].File
		}
		if applied[i].Line != applied[j].Line {
			return applied[i].Line < applied[j].Line
		}
		if applied[i].Method != applied[j].Method {
			return applied[i].Method < applied[j].Method
		}
		return applied[i].Template < applied[j].Template
	})
	return applied, errs
}

func routeContractFiles(root string) (map[string]string, error) {
	ui := filepath.Join(root, "ui", "src")
	out := map[string]string{}
	err := filepath.WalkDir(ui, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "__tests__" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".test.ts") || strings.HasSuffix(name, ".spec.ts") ||
			strings.HasSuffix(name, ".test.vue") || strings.HasSuffix(name, ".spec.vue") ||
			strings.HasSuffix(name, ".d.ts") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch filepath.Ext(name) {
		case ".js", ".mjs", ".cjs", ".jsx":
			// The scanner reads only TS and Vue; a JS file would be skipped silently.
			return fmt.Errorf("%s: unsupported source extension; the route contract scans only .ts, .tsx and .vue", rel)
		}
		if !strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".vue") && !strings.HasSuffix(name, ".tsx") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = string(body)
		return nil
	})
	return out, err
}

type vueChunk struct {
	src  string
	line int
}

func vueScriptChunks(file, src string) ([]vueChunk, []routeContractError) {
	var out []vueChunk
	base := 0
	for {
		rel := strings.Index(src[base:], "<script")
		if rel < 0 {
			break
		}
		abs := base + rel
		gt := strings.IndexByte(src[abs:], '>')
		if gt < 0 {
			return nil, []routeContractError{{File: file, Line: byteLine(src, abs), Msg: "unclosed <script> tag"}}
		}
		tag := src[abs : abs+gt]
		if strings.Contains(tag, " src=") || strings.Contains(tag, "\tsrc=") {
			return nil, []routeContractError{{File: file, Line: byteLine(src, abs), Msg: "unsupported external script"}}
		}
		bodyStart := abs + gt + 1
		endRel := strings.Index(src[bodyStart:], "</script>")
		if endRel < 0 {
			return nil, []routeContractError{{File: file, Line: byteLine(src, abs), Msg: "unclosed <script> tag"}}
		}
		out = append(out, vueChunk{
			src:  src[bodyStart : bodyStart+endRel],
			line: byteLine(src, bodyStart),
		})
		base = bodyStart + endRel + len("</script>")
	}
	return out, nil
}

func byteLine(src string, idx int) int {
	if idx < 0 {
		return 1
	}
	if idx > len(src) {
		idx = len(src)
	}
	return 1 + strings.Count(src[:idx], "\n")
}

func loadRouteContractManifest(path string) ([]routeContractManifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var entries []routeContractManifest
	if err := dec.Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func applyRouteContractManifest(calls []routeContractCall, hits []routeContractHit, entries []routeContractManifest, files map[string]string) ([]routeContractCall, []routeContractError) {
	type key struct {
		file string
		line int
		kind string
	}
	byKey := map[key]routeContractManifest{}
	var errs []routeContractError
	for _, entry := range entries {
		k := key{entry.File, entry.Line, entry.Kind}
		if _, dup := byKey[k]; dup {
			errs = append(errs, routeContractError{File: entry.File, Line: entry.Line, Msg: "duplicate transport manifest entry"})
		}
		if entry.Kind != "new-websocket" && entry.Kind != "use-websocket" {
			errs = append(errs, routeContractError{File: entry.File, Line: entry.Line, Msg: "unsupported transport manifest kind " + entry.Kind})
		}
		if entry.Generic && (entry.Method != "" || entry.Template != "") {
			errs = append(errs, routeContractError{File: entry.File, Line: entry.Line, Msg: "generic transport manifest entry also names a route"})
		}
		if !entry.Generic && (entry.Method == "" || entry.Template == "") {
			errs = append(errs, routeContractError{File: entry.File, Line: entry.Line, Msg: "transport manifest entry is missing method or template"})
		}
		byKey[k] = entry
	}
	used := map[key]bool{}
	for _, hit := range hits {
		k := key{hit.File, hit.Line, hit.Kind}
		entry, ok := byKey[k]
		if !ok {
			errs = append(errs, routeContractError{File: hit.File, Line: hit.Line, Msg: "unsupported transport " + hit.Kind + " (not in manifest)"})
			continue
		}
		if used[k] {
			errs = append(errs, routeContractError{File: hit.File, Line: hit.Line, Msg: "transport manifest entry matched twice"})
			continue
		}
		used[k] = true
		if entry.Generic {
			if hit.Arg == "literal" {
				errs = append(errs, routeContractError{File: hit.File, Line: hit.Line, Msg: "generic transport has a url literal; update the manifest"})
			}
			continue
		}
		tmpl, err := normalizeManifestTemplate(entry.Template)
		if err != nil {
			errs = append(errs, routeContractError{File: entry.File, Line: entry.Line, Msg: err.Error()})
			continue
		}
		if hit.Arg == "literal" {
			got, normErr := normalizeLiteralTransport(hit.Literal)
			if normErr != nil || got != tmpl {
				errs = append(errs, routeContractError{File: hit.File, Line: hit.Line, Msg: fmt.Sprintf("transport literal %q does not match manifest %s", hit.Literal, tmpl)})
				continue
			}
		}
		src := files[hit.File]
		if !codeContains(src, tmpl) {
			errs = append(errs, routeContractError{File: hit.File, Line: hit.Line, Msg: "manifest template " + tmpl + " is not in non-comment source"})
		}
		calls = append(calls, routeContractCall{
			File:     hit.File,
			Line:     hit.Line,
			Method:   strings.ToUpper(entry.Method),
			Template: tmpl,
		})
	}
	for k, entry := range byKey {
		if used[k] {
			continue
		}
		errs = append(errs, routeContractError{File: entry.File, Line: entry.Line, Msg: "stale transport manifest entry"})
	}
	return calls, errs
}

func normalizeManifestTemplate(tmpl string) (string, error) {
	tmpl = strings.TrimSpace(tmpl)
	if i := strings.IndexAny(tmpl, "?#"); i >= 0 {
		tmpl = tmpl[:i]
	}
	if !strings.HasPrefix(tmpl, "/") {
		return "", fmt.Errorf("manifest template %q must be an absolute path", tmpl)
	}
	return normalizeRouteTemplate(tmpl), nil
}

func normalizeLiteralTransport(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "ws://") || strings.HasPrefix(lower, "wss://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		slash := strings.Index(raw, "//")
		if slash < 0 {
			return "", fmt.Errorf("bad absolute url")
		}
		rest := raw[slash+2:]
		pathStart := strings.IndexByte(rest, '/')
		if pathStart < 0 {
			return "", fmt.Errorf("absolute url has no path")
		}
		raw = rest[pathStart:]
	}
	return normalizeManifestTemplate(raw)
}

func codeContains(src, needle string) bool {
	return strings.Contains(stripRouteComments(src), needle)
}

func stripRouteComments(src string) string {
	var b strings.Builder
	s := newRouteScanner("", src, 1)
	for !s.eof() && !s.stop {
		c := s.src[s.i]
		if c == '/' && s.peekN(1) == '/' {
			s.i += 2
			for !s.eof() && s.src[s.i] != '\n' {
				s.i++
			}
			b.WriteByte(' ')
			continue
		}
		if c == '/' && s.peekN(1) == '*' {
			s.i += 2
			for !s.eof() && (s.src[s.i] != '*' || s.peekN(1) != '/') {
				b.WriteByte(' ')
				if s.src[s.i] == '\n' {
					b.WriteByte('\n')
				}
				s.i++
			}
			if !s.eof() {
				s.i += 2
			}
			continue
		}
		if c == '\'' || c == '"' {
			start := s.i
			if _, ok := s.readQuoted(); !ok {
				b.Write(s.src[start:])
				break
			}
			b.Write(s.src[start:s.i])
			continue
		}
		if c == '`' {
			start := s.i
			if _, ok := s.scanTemplate(false); !ok {
				b.Write(s.src[start:])
				break
			}
			b.Write(s.src[start:s.i])
			continue
		}
		b.WriteByte(c)
		s.i++
	}
	return b.String()
}

func normalizeCallPath(base, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty path")
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "://") || strings.HasPrefix(raw, "//") {
		return "", fmt.Errorf("absolute URL")
	}
	base = strings.TrimRight(base, "/")
	raw = strings.TrimLeft(raw, "/")
	if raw == "" {
		return base, nil
	}
	return base + "/" + raw, nil
}

func normalizeRouteTemplate(tmpl string) string {
	var b strings.Builder
	for i := 0; i < len(tmpl); i++ {
		if tmpl[i] != '{' {
			b.WriteByte(tmpl[i])
			continue
		}
		end := muxVarEnd(tmpl, i)
		if end < 0 {
			b.WriteByte(tmpl[i])
			continue
		}
		b.WriteString("{param}")
		i = end
	}
	return b.String()
}

func muxVarEnd(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func missingRoutePairs(calls []routeContractCall, routes map[string]struct{}) []routeContractPair {
	known := map[string]struct{}{}
	for key := range routes {
		method, tmpl, ok := strings.Cut(key, " ")
		if !ok {
			continue
		}
		known[strings.ToUpper(method)+" "+normalizeRouteTemplate(tmpl)] = struct{}{}
	}
	seen := map[string]bool{}
	var out []routeContractPair
	for _, call := range calls {
		key := call.Method + " " + call.Template
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, ok := known[key]; ok {
			continue
		}
		out = append(out, routeContractPair{Method: call.Method, Template: call.Template})
	}
	sortRoutePairs(out)
	return out
}

func sortRoutePairs(out []routeContractPair) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Template != out[j].Template {
			return out[i].Template < out[j].Template
		}
		return out[i].Method < out[j].Method
	})
}

// routePrecedenceSamples are tried in order for every {param} of a call
// template. The first value the expected route accepts on its own builds the
// representative request, so a constrained route such as {id:<uuid>} is not
// reported as shadowed by a value it would reject anyway.
var routePrecedenceSamples = []string{"00000000-0000-0000-0000-000000000000", "1", "x"}

func routePrecedence(router *mux.Router, calls []routeContractCall) ([]routeContractPrec, error) {
	expected := map[string]*mux.Route{}
	err := router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tmpl, tmplErr := route.GetPathTemplate()
		methods, methodErr := route.GetMethods()
		if tmplErr != nil || methodErr != nil {
			return nil
		}
		for _, method := range methods {
			key := strings.ToUpper(method) + " " + normalizeRouteTemplate(tmpl)
			if _, ok := expected[key]; !ok {
				expected[key] = route
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var out []routeContractPrec
	for _, call := range calls {
		key := call.Method + " " + call.Template
		if seen[key] {
			continue
		}
		seen[key] = true
		reqPath := call.Template
		if strings.Contains(call.Template, "{param}") {
			want, ok := expected[key]
			if !ok {
				// No such pair; missingRoutePairs reports it.
				continue
			}
			reqPath = ""
			for _, sample := range routePrecedenceSamples {
				p := strings.ReplaceAll(call.Template, "{param}", sample)
				var m mux.RouteMatch
				if want.Match(httptest.NewRequest(call.Method, p, nil), &m) && m.MatchErr == nil {
					reqPath = p
					break
				}
			}
			if reqPath == "" {
				return nil, fmt.Errorf("%s %s: no sample value satisfies the route's own constraints; extend routePrecedenceSamples", call.Method, call.Template)
			}
		}
		req := httptest.NewRequest(call.Method, reqPath, nil)
		var match mux.RouteMatch
		if !router.Match(req, &match) || match.MatchErr != nil || match.Route == nil {
			continue
		}
		tmpl, err := match.Route.GetPathTemplate()
		if err != nil {
			return nil, fmt.Errorf("%s %s matched a route without a template: %w", call.Method, call.Template, err)
		}
		got := normalizeRouteTemplate(tmpl)
		if got == call.Template {
			continue
		}
		out = append(out, routeContractPrec{
			Method:   call.Method,
			Request:  call.Template,
			Expected: call.Template,
			Matched:  got,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Request != out[j].Request {
			return out[i].Request < out[j].Request
		}
		return out[i].Method < out[j].Method
	})
	return out, nil
}

func checkRouteBaseline(kind string, actual []routeContractPair, baseline []routeContractBaseline) error {
	var problems []string
	base := map[string]routeContractBaseline{}
	for _, entry := range baseline {
		if entry.Issue <= 0 {
			problems = append(problems, fmt.Sprintf("%s baseline entry %s %s has no issue number", kind, entry.Method, entry.Template))
		}
		if entry.Method != strings.ToUpper(entry.Method) || normalizeRouteTemplate(entry.Template) != entry.Template {
			problems = append(problems, fmt.Sprintf("%s baseline entry %s %s is not normalized", kind, entry.Method, entry.Template))
		}
		key := entry.Method + " " + entry.Template
		if _, dup := base[key]; dup {
			problems = append(problems, "duplicate "+kind+" baseline entry "+key)
		}
		base[key] = entry
	}
	act := map[string]routeContractPair{}
	for _, pair := range actual {
		act[pair.Method+" "+pair.Template] = pair
	}
	for key, entry := range base {
		if _, ok := act[key]; ok {
			continue
		}
		problems = append(problems, fmt.Sprintf("stale %s baseline entry %s (issue %d)", kind, key, entry.Issue))
	}
	for key := range act {
		if _, ok := base[key]; ok {
			continue
		}
		problems = append(problems, "missing from "+kind+" baseline: "+key)
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s", strings.Join(problems, "\n"))
}

func checkPrecBaseline(actual []routeContractPrec, baseline []routeContractPrecBaseline) error {
	var problems []string
	type precKey struct{ method, request, matched string }
	base := map[precKey]routeContractPrecBaseline{}
	for _, entry := range baseline {
		if entry.Issue <= 0 {
			problems = append(problems, fmt.Sprintf("precedence baseline %s %s has no issue number", entry.Method, entry.Request))
		}
		if entry.Expected == "" || entry.Expected == entry.Matched {
			problems = append(problems, fmt.Sprintf("precedence baseline %s %s is not a mismatch", entry.Method, entry.Request))
		}
		key := precKey{entry.Method, entry.Request, entry.Matched}
		if _, dup := base[key]; dup {
			problems = append(problems, fmt.Sprintf("duplicate precedence baseline %s %s => %s", entry.Method, entry.Request, entry.Matched))
		}
		base[key] = entry
	}
	act := map[precKey]routeContractPrec{}
	for _, item := range actual {
		act[precKey{item.Method, item.Request, item.Matched}] = item
	}
	for key, entry := range base {
		if _, ok := act[key]; ok {
			continue
		}
		problems = append(problems, fmt.Sprintf("stale precedence baseline %s %s => %s (issue %d)", key.method, key.request, key.matched, entry.Issue))
	}
	for key := range act {
		if _, ok := base[key]; ok {
			continue
		}
		problems = append(problems, fmt.Sprintf("missing from precedence baseline: %s %s => %s", key.method, key.request, key.matched))
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%s", strings.Join(problems, "\n"))
}

func isAPIClientModule(fromFile, spec string) bool {
	spec = strings.TrimSpace(spec)
	var resolved string
	switch {
	case strings.HasPrefix(spec, "@/"):
		resolved = path.Clean("ui/src/" + strings.TrimPrefix(spec, "@/"))
	case strings.HasPrefix(spec, "./"), strings.HasPrefix(spec, "../"):
		dir := path.Dir(filepath.ToSlash(fromFile))
		resolved = path.Clean(dir + "/" + spec)
	default:
		return false
	}
	resolved = strings.TrimSuffix(resolved, ".ts")
	resolved = strings.TrimSuffix(resolved, ".js")
	return resolved == "ui/src/api/client"
}

func isHTTPMethod(method string) bool {
	switch method {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return true
	default:
		return false
	}
}

func looksLikeHTTPPath(raw string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "/") {
		return true
	}
	lower := strings.ToLower(raw)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

type routeScanner struct {
	file         string
	src          []byte
	i            int
	line         int
	canRegex     bool
	member       bool // the previous token was a member-access dot (`.` or `?.`)
	stop         bool
	prevWord     string
	isClientFile bool
	api          map[string]bool
	ws           map[string]bool
	axios        map[string]bool
	calls        []routeContractCall
	hits         []routeContractHit
	errs         []routeContractError
}

type scanMark struct {
	i        int
	line     int
	canRegex bool
	prevWord string
}

func newRouteScanner(file, src string, line int) *routeScanner {
	return &routeScanner{
		file:         file,
		src:          []byte(src),
		line:         line,
		canRegex:     true,
		isClientFile: file == "ui/src/api/client.ts",
		api:          map[string]bool{},
		ws:           map[string]bool{},
		axios:        map[string]bool{},
	}
}

func (s *routeScanner) scanFile() {
	s.canRegex = true
	for !s.stop && !s.eof() {
		s.skipSpaceComments()
		if s.eof() || s.stop {
			return
		}
		s.scanOne()
	}
}

func (s *routeScanner) eof() bool { return s.i >= len(s.src) }

func (s *routeScanner) peek(c byte) bool { return !s.eof() && s.src[s.i] == c }

func (s *routeScanner) peekN(n int) byte {
	i := s.i + n
	if i < 0 || i >= len(s.src) {
		return 0
	}
	return s.src[i]
}

func (s *routeScanner) mark() scanMark {
	return scanMark{s.i, s.line, s.canRegex, s.prevWord}
}

func (s *routeScanner) restore(m scanMark) {
	s.i, s.line, s.canRegex, s.prevWord = m.i, m.line, m.canRegex, m.prevWord
}

func (s *routeScanner) fail(line int, format string, args ...any) {
	if s.stop {
		return
	}
	s.stop = true
	s.errs = append(s.errs, routeContractError{File: s.file, Line: line, Msg: fmt.Sprintf(format, args...)})
}

func (s *routeScanner) skipSpaceComments() {
	for !s.eof() && !s.stop {
		c := s.src[s.i]
		switch c {
		case ' ', '\t', '\r':
			s.i++
		case '\n':
			s.line++
			s.i++
		case '/':
			if s.peekN(1) == '/' {
				s.i += 2
				for !s.eof() && s.src[s.i] != '\n' {
					s.i++
				}
				continue
			}
			if s.peekN(1) == '*' {
				s.skipBlockComment()
				continue
			}
			return
		default:
			return
		}
	}
}

func (s *routeScanner) skipBlockComment() {
	s.i += 2
	for !s.eof() {
		if s.src[s.i] == '*' && s.peekN(1) == '/' {
			s.i += 2
			return
		}
		if s.src[s.i] == '\n' {
			s.line++
		}
		s.i++
	}
	s.fail(s.line, "unterminated block comment")
}

func (s *routeScanner) nextSignificant() byte {
	i := s.i
	for i < len(s.src) {
		c := s.src[i]
		switch c {
		case ' ', '\t', '\r', '\n':
			i++
			continue
		case '/':
			if i+1 < len(s.src) && s.src[i+1] == '/' {
				i += 2
				for i < len(s.src) && s.src[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < len(s.src) && s.src[i+1] == '*' {
				i += 2
				for i+1 < len(s.src) && (s.src[i] != '*' || s.src[i+1] != '/') {
					i++
				}
				if i+1 < len(s.src) {
					i += 2
				}
				continue
			}
			return c
		default:
			return c
		}
	}
	return 0
}

func (s *routeScanner) isIdentStart() bool {
	if s.eof() {
		return false
	}
	c := s.src[s.i]
	if c == '_' || c == '$' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
		return true
	}
	if c < 0x80 {
		return false
	}
	r, _ := utf8.DecodeRune(s.src[s.i:])
	return unicode.IsLetter(r)
}

func (s *routeScanner) isIdentCont(c byte) bool {
	if c == '_' || c == '$' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
		return true
	}
	return c >= 0x80
}

func (s *routeScanner) readIdent() (string, int) {
	if !s.isIdentStart() {
		return "", s.line
	}
	line := s.line
	start := s.i
	_, size := utf8.DecodeRune(s.src[s.i:])
	if size < 1 {
		size = 1
	}
	s.i += size
	for !s.eof() && s.isIdentCont(s.src[s.i]) {
		_, size = utf8.DecodeRune(s.src[s.i:])
		if size < 1 {
			size = 1
		}
		s.i += size
	}
	return string(s.src[start:s.i]), line
}

func (s *routeScanner) peekIdent(want string) bool {
	if !s.isIdentStart() {
		return false
	}
	i := s.i
	_, size := utf8.DecodeRune(s.src[i:])
	if size < 1 {
		size = 1
	}
	i += size
	for i < len(s.src) && s.isIdentCont(s.src[i]) {
		_, size = utf8.DecodeRune(s.src[i:])
		if size < 1 {
			size = 1
		}
		i += size
	}
	return string(s.src[s.i:i]) == want
}

func (s *routeScanner) consumeWord(want string) bool {
	if !s.peekIdent(want) {
		return false
	}
	s.readIdent()
	return true
}

func (s *routeScanner) scanOne() {
	if s.eof() || s.stop {
		return
	}
	c := s.src[s.i]
	member := s.member
	s.member = false
	switch {
	case s.isIdentStart():
		ident, line := s.readIdent()
		s.afterIdent(ident, line, member)
	case c >= '0' && c <= '9':
		s.scanNumber()
	case c == '\'' || c == '"':
		s.readQuoted()
	case c == '`':
		s.scanTemplate(false)
	default:
		s.scanPunct(c)
	}
}

func (s *routeScanner) afterIdent(ident string, line int, member bool) {
	prev := s.prevWord
	s.prevWord = ident
	switch ident {
	case "import":
		if s.nextSignificant() == '.' {
			s.canRegex = false
			return
		}
		s.parseImport()
		return
	case "new":
		s.afterNew()
		return
	case "function", "class", "interface", "enum", "namespace":
		s.canRegex = false
		return
	}
	if ident == "type" && s.looksLikeTypeAlias() {
		s.canRegex = false
		return
	}
	// The client binding has one supported shape: api.<method>(url, ...).
	// Any other use (called, indexed, aliased, passed on) could reach the
	// server without yielding a pair, so it fails. cfg.api is a property.
	if s.api[ident] && !member {
		if !s.tryClientCall(line) {
			s.fail(line, "unsupported use of api client %q; call %s.<method>(url, ...)", ident, ident)
		}
		return
	}
	if ident == "fetch" {
		mark := s.mark()
		s.skipSpaceComments()
		if s.peek('(') {
			s.fail(line, "unsupported call style fetch()")
			return
		}
		s.restore(mark)
		s.prevWord = ident
	}
	if (ident == "useWebSocket" || s.ws[ident]) && prev != "function" && s.looksLikeCall() {
		s.finishTransport(line, "use-websocket")
		return
	}
	if ident == "WebSocket" && s.looksLikeCall() {
		s.fail(line, "unsupported WebSocket() call without new")
		return
	}
	if (ident == "axios" || s.axios[ident]) && s.tryAxios(line) {
		return
	}
	if ident == "XMLHttpRequest" {
		s.fail(line, "unsupported XMLHttpRequest")
		return
	}
	if s.tryForeignHTTP(line) {
		return
	}
	if unaryKeyword(ident) {
		s.canRegex = true
		return
	}
	s.canRegex = false
}

func unaryKeyword(ident string) bool {
	switch ident {
	case "await", "yield", "void", "typeof", "delete", "return", "throw", "case", "else", "do", "in", "of", "instanceof", "extends", "infer":
		return true
	default:
		return false
	}
}

func (s *routeScanner) looksLikeTypeAlias() bool {
	switch s.nextSignificant() {
	case 0, '=', '(', '.', ',', ';', ')', '[', '?', ':', '&', '|':
		return false
	default:
		return true
	}
}

func (s *routeScanner) looksLikeCall() bool {
	switch s.nextSignificant() {
	case '(', '<':
		return true
	default:
		return false
	}
}

func (s *routeScanner) afterNew() {
	mark := s.mark()
	s.skipSpaceComments()
	name, nameLine := s.readIdent()
	if name != "WebSocket" {
		s.restore(mark)
		s.canRegex = true
		s.prevWord = "new"
		return
	}
	if !s.looksLikeCall() {
		s.fail(nameLine, "unsupported WebSocket")
		return
	}
	s.finishTransport(nameLine, "new-websocket")
}

func (s *routeScanner) tryClientCall(line int) bool {
	mark := s.mark()
	if !s.consumeDot() {
		return false
	}
	method, _ := s.readIdent()
	if method == "request" {
		s.fail(line, "unsupported api.request() call")
		return true
	}
	if !isHTTPMethod(method) {
		s.restore(mark)
		return false
	}
	s.skipSpaceComments()
	if s.peek('<') && !s.skipTypeArgs() {
		s.fail(line, "unsupported type arguments on api.%s", method)
		return true
	}
	s.skipSpaceComments()
	if !s.peek('(') {
		s.fail(line, "unsupported api.%s call", method)
		return true
	}
	s.i++
	s.skipSpaceComments()
	raw, ok := s.readPathArg()
	if !ok {
		if !s.stop {
			s.fail(line, "unsupported url argument for %s()", method)
		}
		return true
	}
	if !s.argEnds() {
		s.fail(line, "unsupported url expression for %s(); pass one string or template literal", method)
		return true
	}
	full, err := normalizeCallPath(routeContractAPIBase, raw)
	if err != nil {
		s.fail(line, "unsupported url %q", raw)
		return true
	}
	s.calls = append(s.calls, routeContractCall{
		File:     s.file,
		Line:     line,
		Method:   strings.ToUpper(method),
		Template: full,
	})
	s.canRegex = false
	s.prevWord = ""
	return true
}

func (s *routeScanner) tryAxios(line int) bool {
	mark := s.mark()
	if !s.consumeDot() {
		return false
	}
	method, _ := s.readIdent()
	if method == "create" && s.isClientFile {
		s.canRegex = false
		s.prevWord = ""
		return true
	}
	if isHTTPMethod(method) || method == "request" {
		s.fail(line, "unsupported axios.%s call", method)
		return true
	}
	s.restore(mark)
	return false
}

func (s *routeScanner) tryForeignHTTP(line int) bool {
	mark := s.mark()
	if !s.consumeDot() {
		return false
	}
	method, _ := s.readIdent()
	if !isHTTPMethod(method) {
		s.restore(mark)
		return false
	}
	s.skipSpaceComments()
	if s.peek('<') && !s.skipTypeArgs() {
		s.restore(mark)
		return false
	}
	s.skipSpaceComments()
	if !s.peek('(') {
		s.restore(mark)
		return false
	}
	s.i++
	s.skipSpaceComments()
	if s.pathArgAhead() {
		s.fail(line, "unsupported http call .%s()", method)
		return true
	}
	s.restore(mark)
	return false
}

func (s *routeScanner) pathArgAhead() bool {
	if s.peek('\'') || s.peek('"') {
		mark := s.mark()
		text, ok := s.readQuoted()
		s.restore(mark)
		return ok && looksLikeHTTPPath(text)
	}
	if !s.peek('`') {
		return false
	}
	i := s.i + 1
	var b strings.Builder
	for i < len(s.src) {
		c := s.src[i]
		if c == '\\' {
			i += 2
			continue
		}
		if c == '`' || (c == '$' && i+1 < len(s.src) && s.src[i+1] == '{') {
			break
		}
		b.WriteByte(c)
		i++
	}
	return looksLikeHTTPPath(b.String())
}

func (s *routeScanner) consumeDot() bool {
	s.skipSpaceComments()
	if s.peek('?') && s.peekN(1) == '.' {
		s.i += 2
		return true
	}
	if s.peek('.') {
		s.i++
		return true
	}
	return false
}

func (s *routeScanner) finishTransport(line int, kind string) {
	s.skipSpaceComments()
	if s.peek('<') && !s.skipTypeArgs() {
		s.fail(line, "unsupported type arguments on %s", kind)
		return
	}
	s.skipSpaceComments()
	if !s.peek('(') {
		s.fail(line, "unsupported %s", kind)
		return
	}
	s.i++
	s.skipSpaceComments()
	hit := routeContractHit{File: s.file, Line: line, Kind: kind}
	switch {
	case s.peek('\'') || s.peek('"') || s.peek('`'):
		raw, ok := s.readPathArg()
		if !ok {
			if !s.stop {
				s.fail(line, "unsupported transport url")
			}
			return
		}
		if !s.argEnds() {
			s.fail(line, "unsupported transport url expression")
			return
		}
		hit.Arg = "literal"
		hit.Literal = raw
	case s.isIdentStart():
		s.readIdent()
		hit.Arg = "ident"
	case s.peek('{'):
		hit.Arg = "object"
	default:
		hit.Arg = "other"
	}
	s.hits = append(s.hits, hit)
	s.canRegex = true
	s.prevWord = ""
}

// argEnds reports whether the url argument just read is the whole argument:
// the next significant byte must close the call or start the next argument.
// Otherwise '/devices' + suffix would be read as /devices.
func (s *routeScanner) argEnds() bool {
	mark := s.mark()
	s.skipSpaceComments()
	ok := s.peek(',') || s.peek(')')
	s.restore(mark)
	return ok
}

func (s *routeScanner) readPathArg() (string, bool) {
	if s.peek('\'') || s.peek('"') {
		return s.readQuoted()
	}
	if s.peek('`') {
		return s.scanTemplate(true)
	}
	return "", false
}

func (s *routeScanner) skipTypeArgs() bool {
	if !s.peek('<') {
		return true
	}
	s.i++
	depth := 1
	for !s.eof() && depth > 0 && !s.stop {
		c := s.src[s.i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			s.i++
		case c == '\n':
			s.line++
			s.i++
		case c == '/' && s.peekN(1) == '/':
			s.i += 2
			for !s.eof() && s.src[s.i] != '\n' {
				s.i++
			}
		case c == '/' && s.peekN(1) == '*':
			s.skipBlockComment()
		case c == '\'' || c == '"':
			if _, ok := s.readQuoted(); !ok {
				return false
			}
		case c == '`':
			if _, ok := s.scanTemplate(false); !ok {
				return false
			}
		case c == '=' && s.peekN(1) == '>':
			s.i += 2
		case c == '<':
			depth++
			s.i++
		case c == '>':
			depth--
			s.i++
		default:
			s.i++
		}
	}
	return depth == 0
}

func (s *routeScanner) readQuoted() (string, bool) {
	if s.eof() {
		return "", false
	}
	quote := s.src[s.i]
	if quote != '\'' && quote != '"' {
		return "", false
	}
	s.i++
	var b strings.Builder
	for !s.eof() {
		c := s.src[s.i]
		if c == '\n' {
			s.fail(s.line, "unterminated string")
			return "", false
		}
		if c == '\\' {
			s.i++
			if s.eof() {
				s.fail(s.line, "unterminated string")
				return "", false
			}
			esc := s.src[s.i]
			s.i++
			if esc == '\n' {
				s.line++
				continue
			}
			decoded, extra, ok := s.decodeEscape(esc)
			if !ok {
				return "", false
			}
			s.i += extra
			b.WriteString(decoded)
			continue
		}
		if c == quote {
			s.i++
			s.canRegex = false
			return b.String(), true
		}
		s.i++
		b.WriteByte(c)
	}
	s.fail(s.line, "unterminated string")
	return "", false
}

func (s *routeScanner) decodeEscape(esc byte) (string, int, bool) {
	switch esc {
	case 'n':
		return "\n", 0, true
	case 'r':
		return "\r", 0, true
	case 't':
		return "\t", 0, true
	case 'b':
		return "\b", 0, true
	case 'f':
		return "\f", 0, true
	case 'v':
		return "\v", 0, true
	case '0':
		return "\x00", 0, true
	case '\\', '\'', '"', '`', '/':
		return string(esc), 0, true
	case 'u':
		if s.peek('{') {
			end := 1
			for s.i+end < len(s.src) && s.src[s.i+end] != '}' {
				end++
				if end > 8 {
					s.fail(s.line, "bad unicode escape")
					return "", 0, false
				}
			}
			if s.i+end >= len(s.src) || s.src[s.i+end] != '}' || end < 2 {
				s.fail(s.line, "bad unicode escape")
				return "", 0, false
			}
			n, err := strconv.ParseUint(string(s.src[s.i+1:s.i+end]), 16, 32)
			if err != nil || !utf8.ValidRune(rune(n)) {
				s.fail(s.line, "bad unicode escape")
				return "", 0, false
			}
			return string(rune(n)), end + 1, true
		}
		if s.i+4 > len(s.src) {
			s.fail(s.line, "bad unicode escape")
			return "", 0, false
		}
		n, err := strconv.ParseUint(string(s.src[s.i:s.i+4]), 16, 32)
		if err != nil || !utf8.ValidRune(rune(n)) {
			s.fail(s.line, "bad unicode escape")
			return "", 0, false
		}
		return string(rune(n)), 4, true
	case 'x':
		if s.i+2 > len(s.src) {
			s.fail(s.line, "bad hex escape")
			return "", 0, false
		}
		n, err := strconv.ParseUint(string(s.src[s.i:s.i+2]), 16, 8)
		if err != nil {
			s.fail(s.line, "bad hex escape")
			return "", 0, false
		}
		return string(rune(n)), 2, true
	default:
		return string(esc), 0, true
	}
}

func (s *routeScanner) scanTemplate(pathMode bool) (string, bool) {
	if !s.peek('`') {
		return "", false
	}
	s.i++
	var b strings.Builder
	for !s.stop {
		if s.eof() {
			s.fail(s.line, "unterminated template")
			return "", false
		}
		c := s.src[s.i]
		if c == '\\' {
			s.i++
			if s.eof() {
				s.fail(s.line, "unterminated template")
				return "", false
			}
			esc := s.src[s.i]
			s.i++
			if esc == '\n' {
				s.line++
				continue
			}
			decoded, extra, ok := s.decodeEscape(esc)
			if !ok {
				return "", false
			}
			s.i += extra
			if pathMode {
				b.WriteString(decoded)
			}
			continue
		}
		if c == '`' {
			s.i++
			s.canRegex = false
			return b.String(), true
		}
		if c == '$' && s.peekN(1) == '{' {
			s.i += 2
			if pathMode {
				if !s.readSimpleInterp(&b) {
					return "", false
				}
			} else if !s.scanInterp() {
				return "", false
			}
			continue
		}
		if c == '\n' {
			s.line++
		}
		if pathMode {
			b.WriteByte(c)
		}
		s.i++
	}
	return "", false
}

func (s *routeScanner) readSimpleInterp(b *strings.Builder) bool {
	for !s.stop {
		s.skipSpaceComments()
		if s.eof() {
			s.fail(s.line, "unterminated interpolation")
			return false
		}
		if s.peek('}') {
			s.i++
			b.WriteString("{param}")
			return true
		}
		if s.isIdentStart() {
			s.readIdent()
			continue
		}
		if s.peek('?') && s.peekN(1) == '.' {
			s.i += 2
			continue
		}
		if s.peek('.') {
			s.i++
			continue
		}
		s.fail(s.line, "unsupported template url expression")
		return false
	}
	return false
}

func (s *routeScanner) scanInterp() bool {
	saved := s.canRegex
	s.canRegex = true
	depth := 0
	for !s.stop {
		s.skipSpaceComments()
		if s.eof() {
			s.fail(s.line, "unterminated interpolation")
			return false
		}
		if s.peek('{') {
			depth++
			s.i++
			s.canRegex = true
			s.prevWord = ""
			continue
		}
		if s.peek('}') {
			if depth == 0 {
				s.i++
				s.canRegex = saved
				return true
			}
			depth--
			s.i++
			s.canRegex = true
			s.prevWord = ""
			continue
		}
		s.scanOne()
	}
	return false
}

func (s *routeScanner) parseImport() {
	s.skipSpaceComments()
	if s.peek('(') {
		s.parseDynamicImport()
		return
	}
	if s.peek('\'') || s.peek('"') {
		spec, ok := s.readQuoted()
		if !ok {
			if !s.stop {
				s.fail(s.line, "bad import specifier")
			}
			return
		}
		s.noteModule(nil, spec)
		s.optionalSemi()
		return
	}
	typeOnly := false
	if s.peekIdent("type") {
		mark := s.mark()
		s.readIdent()
		s.skipSpaceComments()
		if s.peek('{') || s.peek('*') || (s.isIdentStart() && !s.peekIdent("from")) {
			typeOnly = true
		} else {
			s.restore(mark)
		}
	}
	var bindings []importBinding
	switch {
	case s.peek('*'):
		s.i++
		s.skipSpaceComments()
		if !s.consumeWord("as") {
			s.fail(s.line, "import * without as")
			return
		}
		s.skipSpaceComments()
		local, _ := s.readIdent()
		if local == "" {
			s.fail(s.line, "import * without binding")
			return
		}
		bindings = append(bindings, importBinding{local: local, imported: "*"})
	case s.peek('{'):
		named, ok := s.parseNamedImports()
		if !ok {
			return
		}
		bindings = named
	case s.isIdentStart():
		local, _ := s.readIdent()
		bindings = append(bindings, importBinding{local: local, imported: "default"})
		s.skipSpaceComments()
		if s.peek(',') {
			s.i++
			s.skipSpaceComments()
			named, ok := s.parseNamedImports()
			if !ok {
				return
			}
			bindings = append(bindings, named...)
		}
	default:
		s.fail(s.line, "unsupported import syntax")
		return
	}
	s.skipSpaceComments()
	if !s.consumeWord("from") {
		s.fail(s.line, "import without from")
		return
	}
	s.skipSpaceComments()
	spec, ok := s.readQuoted()
	if !ok {
		if !s.stop {
			s.fail(s.line, "import without specifier")
		}
		return
	}
	if !typeOnly {
		s.noteModule(bindings, spec)
	}
	s.optionalSemi()
	s.canRegex = true
	s.prevWord = ""
}

type importBinding struct {
	local    string
	imported string
}

func (s *routeScanner) parseNamedImports() ([]importBinding, bool) {
	if !s.peek('{') {
		s.fail(s.line, "expected named imports")
		return nil, false
	}
	s.i++
	var out []importBinding
	for !s.stop {
		s.skipSpaceComments()
		if s.peek('}') {
			s.i++
			return out, true
		}
		inlineType := false
		if s.peekIdent("type") {
			mark := s.mark()
			s.readIdent()
			s.skipSpaceComments()
			if s.peek(',') || s.peek('}') || s.peekIdent("as") {
				s.restore(mark)
			} else {
				inlineType = true
			}
		}
		imported, _ := s.readIdent()
		if imported == "" {
			s.fail(s.line, "bad named import")
			return nil, false
		}
		local := imported
		s.skipSpaceComments()
		if s.consumeWord("as") {
			s.skipSpaceComments()
			local, _ = s.readIdent()
			if local == "" {
				s.fail(s.line, "bad import alias")
				return nil, false
			}
		}
		if !inlineType {
			out = append(out, importBinding{local: local, imported: imported})
		}
		s.skipSpaceComments()
		if s.peek(',') {
			s.i++
			continue
		}
		if s.peek('}') {
			s.i++
			return out, true
		}
		s.fail(s.line, "bad named import")
		return nil, false
	}
	return nil, false
}

func (s *routeScanner) parseDynamicImport() {
	line := s.line
	s.i++
	s.skipSpaceComments()
	spec, ok := s.readQuoted()
	if !ok {
		if !s.stop {
			s.fail(line, "unsupported dynamic import")
		}
		return
	}
	s.skipSpaceComments()
	if !s.peek(')') {
		s.fail(line, "unsupported dynamic import")
		return
	}
	s.i++
	if isAPIClientModule(s.file, spec) || spec == "axios" || forbiddenHTTPModule(spec) {
		s.fail(line, "unsupported dynamic import of %s", spec)
		return
	}
	s.canRegex = false
	s.prevWord = ""
}

func (s *routeScanner) noteModule(bindings []importBinding, spec string) {
	if isAPIClientModule(s.file, spec) {
		for _, binding := range bindings {
			switch binding.imported {
			case "default", "api":
				s.api[binding.local] = true
			default:
				s.fail(s.line, "unsupported client import %q", binding.imported)
				return
			}
		}
		return
	}
	if spec == "axios" {
		if !s.isClientFile {
			s.fail(s.line, "unsupported axios import")
			return
		}
		for _, binding := range bindings {
			if binding.imported == "default" {
				s.axios[binding.local] = true
			}
		}
		return
	}
	if forbiddenHTTPModule(spec) {
		s.fail(s.line, "unsupported http import %q", spec)
		return
	}
	for _, binding := range bindings {
		if binding.imported == "useWebSocket" {
			s.ws[binding.local] = true
		}
	}
}

func forbiddenHTTPModule(spec string) bool {
	switch spec {
	case "ky", "got", "node-fetch", "cross-fetch", "isomorphic-fetch", "websocket", "http", "https", "node:http", "node:https":
		return true
	default:
		return false
	}
}

func (s *routeScanner) optionalSemi() {
	s.skipSpaceComments()
	if s.peek(';') {
		s.i++
	}
}

func (s *routeScanner) scanNumber() {
	if s.peek('0') && (s.peekN(1) == 'x' || s.peekN(1) == 'X' || s.peekN(1) == 'b' || s.peekN(1) == 'B' || s.peekN(1) == 'o' || s.peekN(1) == 'O') {
		s.i += 2
		for !s.eof() && (isHex(s.src[s.i]) || s.src[s.i] == '_') {
			s.i++
		}
	} else {
		for !s.eof() {
			c := s.src[s.i]
			if (c >= '0' && c <= '9') || c == '_' || c == '.' {
				s.i++
				continue
			}
			if c == 'e' || c == 'E' {
				s.i++
				if s.peek('+') || s.peek('-') {
					s.i++
				}
				continue
			}
			break
		}
	}
	if s.peek('n') {
		s.i++
	}
	s.canRegex = false
	s.prevWord = ""
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func (s *routeScanner) scanPunct(c byte) {
	switch c {
	case '(', '[', '{':
		s.i++
		s.canRegex = true
		s.prevWord = ""
	case ')', ']':
		s.i++
		s.canRegex = false
		s.prevWord = ""
	case '}':
		s.i++
		s.canRegex = true
		s.prevWord = ""
	case ',', ';', ':', '~':
		s.i++
		s.canRegex = true
		s.prevWord = ""
	case '?':
		s.scanQuestion()
	case '!':
		s.scanBang()
	case '=':
		s.scanEq()
	case '<':
		s.scanAngle(true)
	case '>':
		s.scanAngle(false)
	case '+', '-':
		s.scanPlusMinus(c)
	case '*', '%', '&', '|', '^':
		s.scanOp(c)
	case '.':
		s.scanDot()
	case '/':
		s.scanSlash()
	case '@':
		s.i++
		s.canRegex = true
		s.prevWord = ""
	case '#':
		s.i++
		if s.isIdentStart() {
			s.readIdent()
		}
		s.canRegex = false
		s.prevWord = ""
	default:
		if c >= 0x80 {
			r, size := utf8.DecodeRune(s.src[s.i:])
			s.fail(s.line, "unsupported character %q", r)
			if size < 1 {
				size = 1
			}
			s.i += size
			return
		}
		s.fail(s.line, "unsupported character %q", string(c))
	}
}

func (s *routeScanner) scanQuestion() {
	s.i++
	if s.peek('?') {
		s.i++
		s.canRegex = true
		s.prevWord = ""
		return
	}
	if s.peek('.') && (s.peekN(1) < '0' || s.peekN(1) > '9') {
		// The current byte is '.', so the digit check is peek of the byte after '.'.
		next := byte(0)
		if s.i+1 < len(s.src) {
			next = s.src[s.i+1]
		}
		if next < '0' || next > '9' {
			s.i++
			s.canRegex = false
			s.prevWord = ""
			s.member = true
			return
		}
	}
	s.canRegex = true
	s.prevWord = ""
}

func (s *routeScanner) scanBang() {
	prefix := s.canRegex
	s.i++
	if s.peek('=') {
		s.i++
		if s.peek('=') {
			s.i++
		}
		s.canRegex = true
		s.prevWord = ""
		return
	}
	s.canRegex = prefix
	s.prevWord = ""
}

func (s *routeScanner) scanEq() {
	s.i++
	if s.peek('>') {
		s.i++
		s.canRegex = true
		s.prevWord = ""
		return
	}
	if s.peek('=') {
		s.i++
		if s.peek('=') {
			s.i++
		}
	}
	s.canRegex = true
	s.prevWord = ""
}

func (s *routeScanner) scanAngle(lt bool) {
	s.i++
	if s.peek('=') {
		s.i++
		s.canRegex = true
		s.prevWord = ""
		return
	}
	op := byte('>')
	if lt {
		op = '<'
	}
	if s.peek(op) {
		s.i++
		if s.peek(op) {
			s.i++
		}
		if s.peek('=') {
			s.i++
		}
	}
	s.canRegex = true
	s.prevWord = ""
}

func (s *routeScanner) scanPlusMinus(c byte) {
	prefix := s.canRegex
	s.i++
	if s.peek(c) {
		s.i++
		s.canRegex = prefix
		s.prevWord = ""
		return
	}
	if s.peek('=') {
		s.i++
	}
	s.canRegex = true
	s.prevWord = ""
}

func (s *routeScanner) scanOp(c byte) {
	s.i++
	if c == '*' && s.peek('*') {
		s.i++
	}
	if c == '&' && s.peek('&') {
		s.i++
	}
	if c == '|' && s.peek('|') {
		s.i++
	}
	if s.peek('=') {
		s.i++
	}
	s.canRegex = true
	s.prevWord = ""
}

func (s *routeScanner) scanDot() {
	if s.peekN(1) == '.' && s.peekN(2) == '.' {
		s.i += 3
		s.canRegex = true
		s.prevWord = ""
		return
	}
	if s.peekN(1) >= '0' && s.peekN(1) <= '9' {
		s.scanNumber()
		return
	}
	s.i++
	s.canRegex = false
	s.prevWord = ""
	s.member = true
}

func (s *routeScanner) scanSlash() {
	if s.canRegex {
		s.scanRegex()
		return
	}
	s.i++
	if s.peek('=') {
		s.i++
	}
	s.canRegex = true
	s.prevWord = ""
}

func (s *routeScanner) scanRegex() {
	s.i++
	inClass := false
	for {
		if s.eof() {
			s.fail(s.line, "unterminated regex")
			return
		}
		c := s.src[s.i]
		if c == '\\' {
			s.i++
			if s.eof() {
				s.fail(s.line, "unterminated regex")
				return
			}
			if s.src[s.i] == '\n' {
				s.line++
			}
			s.i++
			continue
		}
		if c == '\n' {
			s.fail(s.line, "unterminated regex")
			return
		}
		if c == '[' && !inClass {
			inClass = true
			s.i++
			continue
		}
		if c == ']' && inClass {
			inClass = false
			s.i++
			continue
		}
		if c == '/' && !inClass {
			s.i++
			for !s.eof() && isRegexFlag(s.src[s.i]) {
				s.i++
			}
			s.canRegex = false
			s.prevWord = ""
			return
		}
		s.i++
	}
}

func isRegexFlag(c byte) bool {
	switch c {
	case 'g', 'i', 'm', 's', 'u', 'y', 'd':
		return true
	default:
		return false
	}
}
