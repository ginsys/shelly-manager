package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
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

// routeContractAPIBase must equal the default baseURL in client.ts;
// checkClientBase fails the scan when it does not. The VITE_API_BASE and
// window.__API_BASE__ overrides are deployment choices outside the contract.
const routeContractAPIBase = "/api/v1"

const routeContractClientFile = "ui/src/api/client.ts"

// checkClientBase reads the axios instance's baseURL in client.ts and fails
// unless it has exactly one, whose final fallback operand is the literal
// routeContractAPIBase.
func checkClientBase(src string) []routeContractError {
	code := stripRouteComments(src)
	re := regexp.MustCompile(`\bbaseURL\b`)
	locs := re.FindAllStringIndex(code, -1)
	if len(locs) != 1 {
		return []routeContractError{{File: routeContractClientFile, Line: 1, Msg: fmt.Sprintf("want exactly one baseURL, found %d", len(locs))}}
	}
	line := byteLine(code, locs[0][0])
	// The value runs to the property's end, across lines. Only a chain of
	// plain operands (names, member paths, string literals) joined by || or
	// ?? is read: anything else (a call, a ternary, concatenation) could
	// change the base, so it fails rather than being partly read.
	shape := "unsupported baseURL shape; want baseURL: <overrides> || '<base>'"
	m := regexp.MustCompile(`^baseURL\s*:\s*([^,}]+)[,}]`).FindStringSubmatch(code[locs[0][0]:])
	if m == nil {
		return []routeContractError{{File: routeContractClientFile, Line: line, Msg: shape}}
	}
	operand := regexp.MustCompile(`^(?:[\w$]+(?:\??\.[\w$]+)*|'[^'\\]*'|"[^"\\]*")$`)
	ops := regexp.MustCompile(`\|\||\?\?`).Split(m[1], -1)
	for _, op := range ops {
		if !operand.MatchString(strings.TrimSpace(op)) {
			return []routeContractError{{File: routeContractClientFile, Line: line, Msg: shape}}
		}
	}
	last := strings.TrimSpace(ops[len(ops)-1])
	want := []string{"'" + routeContractAPIBase + "'", `"` + routeContractAPIBase + `"`}
	if !slices.Contains(want, last) {
		return []routeContractError{{File: routeContractClientFile, Line: line, Msg: fmt.Sprintf("default baseURL %s does not match routeContractAPIBase %q", last, routeContractAPIBase)}}
	}
	return nil
}

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
	if _, ok := files[routeContractClientFile]; !ok {
		return nil, []routeContractError{{File: routeContractClientFile, Line: 1, Msg: "api client module not found"}}
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
	if src, ok := files[routeContractClientFile]; ok {
		errs = append(errs, checkClientBase(src)...)
	}
	for _, name := range names {
		src := files[name]
		if strings.HasSuffix(name, ".vue") {
			chunks, chunkErrs := vueScriptChunks(name, src)
			errs = append(errs, chunkErrs...)
			// <script> and <script setup> share one module scope: an import
			// in either is usable in both, so every block is scanned with
			// the bindings all of them declare.
			var seed []*routeScanner
			for _, chunk := range chunks {
				sc := newRouteScanner(name, chunk.src, chunk.line)
				sc.scanFile()
				seed = append(seed, sc)
			}
			bindings := map[string]bool{}
			for _, chunk := range chunks {
				sc := newRouteScanner(name, chunk.src, chunk.line)
				for _, p := range seed {
					maps.Copy(sc.api, p.api)
					maps.Copy(sc.ws, p.ws)
					maps.Copy(sc.axios, p.axios)
				}
				sc.scanFile()
				calls = append(calls, sc.calls...)
				hits = append(hits, sc.hits...)
				errs = append(errs, sc.errs...)
				for _, m := range []map[string]bool{sc.api, sc.ws, sc.axios} {
					for b := range m {
						bindings[b] = true
					}
				}
			}
			errs = append(errs, checkVueTemplate(name, src, chunks, bindings)...)
			continue
		}
		sc := newRouteScanner(name, src, 1)
		sc.scanFile()
		// checkClientBase validates the one baseURL key, which pairs calls
		// only if axios.create is what builds the client: exactly one.
		if sc.isClientFile && !sc.created && len(sc.errs) == 0 {
			sc.fail(1, "the api client module creates no axios instance; create it with axios.create(...)")
		}
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
	src        string
	line       int
	start, end int // byte range of src within the file
}

// checkVueTemplate fails when the parts of a .vue file outside its <script>
// blocks name a client or transport binding the scripts imported: template
// expressions (@click="api.get(...)") run with the script setup bindings in
// scope and are not scanned.
func checkVueTemplate(file, src string, chunks []vueChunk, bindings map[string]bool) []routeContractError {
	outside := []byte(src)
	for _, chunk := range chunks {
		for i := chunk.start; i < chunk.end; i++ {
			if outside[i] != '\n' {
				outside[i] = ' '
			}
		}
	}
	names := make([]string, 0, len(bindings))
	for name := range bindings {
		names = append(names, name)
	}
	sort.Strings(names)
	// Vue has many expression forms (quoted and unquoted directive values,
	// .prop and dynamic-argument shorthands, {{ }}), so the template is not
	// parsed: the binding named as a word anywhere outside <script> fails,
	// prose and class names included. Rename the binding or move the call
	// into the script. Only a member access (cfg.api, cfg?.api) of the same
	// word is something else; a spread (...api) is the binding.
	var errs []routeContractError
	for _, name := range names {
		// Identifier characters are _, $, letters and digits; any other
		// rune (spaces, U+2028, U+00A0, punctuation) ends the name.
		identRune := func(r rune) bool {
			return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
		}
		for off := 0; ; {
			rel := bytes.Index(outside[off:], []byte(name))
			if rel < 0 {
				break
			}
			at, end := off+rel, off+rel+len(name)
			off = at + 1
			if before, _ := utf8.DecodeLastRune(outside[:at]); at > 0 && identRune(before) {
				continue
			}
			if after, _ := utf8.DecodeRune(outside[end:]); end < len(outside) && identRune(after) {
				continue
			}
			if at > 0 && outside[at-1] == '.' && (at < 3 || string(outside[at-3:at]) != "...") {
				continue
			}
			errs = append(errs, routeContractError{File: file, Line: byteLine(src, at), Msg: fmt.Sprintf("unsupported use of %q outside <script>; templates are not scanned", name)})
			break
		}
	}
	return errs
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
			src:   src[bodyStart : bodyStart+endRel],
			line:  byteLine(src, bodyStart),
			start: bodyStart,
			end:   bodyStart + endRel,
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
		if !codeContainsPath(src, tmpl) {
			errs = append(errs, routeContractError{File: hit.File, Line: hit.Line, Msg: "manifest template " + tmpl + " is not a whole path in non-comment source"})
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

// codeContainsPath reports whether the path appears in non-comment source as
// a whole path: starting after a quote or an interpolation (`${...}`) and
// ending at a quote, a query, a fragment or an interpolation. A longer path
// that merely contains it (/metrics/ws-broken, /x/metrics/ws) does not count.
func codeContainsPath(src, path string) bool {
	code := stripRouteComments(src)
	for off := 0; ; {
		rel := strings.Index(code[off:], path)
		if rel < 0 {
			return false
		}
		at, end := off+rel, off+rel+len(path)
		off = at + 1
		if at == 0 || !strings.ContainsRune("'\"`}", rune(code[at-1])) {
			continue
		}
		if end < len(code) && strings.ContainsRune("'\"`?#$", rune(code[end])) {
			return true
		}
	}
}

// lineCommentEnd returns the index of the line terminator that ends a //
// comment whose text starts at i, or len(src). JavaScript ends it at LF, CR,
// U+2028 and U+2029; code after any of them runs.
func lineCommentEnd(src []byte, i int) int {
	for ; i < len(src); i++ {
		switch src[i] {
		case '\n', '\r':
			return i
		case 0xE2:
			if i+2 < len(src) && src[i+1] == 0x80 && (src[i+2] == 0xA8 || src[i+2] == 0xA9) {
				return i
			}
		}
	}
	return i
}

func stripRouteComments(src string) string {
	var b strings.Builder
	s := newRouteScanner("", src, 1)
	for !s.eof() && !s.stop {
		c := s.src[s.i]
		if c == '/' && s.peekN(1) == '/' {
			s.i += 2
			s.i = lineCommentEnd(s.src, s.i)
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

// routePrecedenceSamples are the candidate values for each {param} of a call
// template. Every combination is tried, one sample per parameter, and each
// one the expected route accepts on its own is a representative request. A
// constrained route such as {id:<uuid>} is therefore not reported as shadowed
// by a value it would reject anyway, parameters with disjoint constraints
// ({id:[0-9]+}/{section:[a-z]+}) each get a value that fits, and an earlier
// route that catches only some accepted values (/devices/1/...) still shows.
var routePrecedenceSamples = []string{"00000000-0000-0000-0000-000000000000", "1", "x"}

// routePrecedenceRequests returns every sample path the expected route
// matches, filling each {param} of tmpl independently.
func routePrecedenceRequests(method, tmpl string, want *mux.Route) []string {
	parts := strings.Split(tmpl, "{param}")
	choice := make([]int, len(parts)-1)
	var out []string
	for {
		var b strings.Builder
		for i, part := range parts {
			b.WriteString(part)
			if i < len(choice) {
				b.WriteString(routePrecedenceSamples[choice[i]])
			}
		}
		p := b.String()
		var m mux.RouteMatch
		if want.Match(httptest.NewRequest(method, p, nil), &m) && m.MatchErr == nil {
			out = append(out, p)
		}
		// Advance the odometer over sample indexes; done after the last one.
		i := len(choice) - 1
		for i >= 0 && choice[i] == len(routePrecedenceSamples)-1 {
			choice[i] = 0
			i--
		}
		if i < 0 {
			return out
		}
		choice[i]++
	}
}

func routePrecedence(router *mux.Router, calls []routeContractCall) ([]routeContractPrec, error) {
	expected := map[string]*mux.Route{}
	served := map[string][]string{} // raw templates of every route per pair
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
			served[key] = append(served[key], tmpl)
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
		if routes := served[key]; len(routes) > 1 {
			// Which route a request reaches depends on its values, so no
			// single route is the expected one.
			return nil, fmt.Errorf("%s %s is served by %d routes (%s); make the templates distinct", call.Method, call.Template, len(routes), strings.Join(routes, ", "))
		}
		reqPaths := []string{call.Template}
		want := expected[key]
		if strings.Contains(call.Template, "{param}") {
			if want == nil {
				// No such pair; missingRoutePairs reports it.
				continue
			}
			reqPaths = routePrecedenceRequests(call.Method, call.Template, want)
			if len(reqPaths) == 0 {
				return nil, fmt.Errorf("%s %s: no sample value satisfies the route's own constraints; extend routePrecedenceSamples", call.Method, call.Template)
			}
		}
		// Every distinct route that catches a sample request is reported,
		// so a new shadow cannot hide behind a baselined one. A shadow is
		// named by its raw template: {id:[0-9]+} and {id:[a-z]+} are two
		// routes even though both normalize to {param}.
		matched := map[string]bool{}
		for _, reqPath := range reqPaths {
			req := httptest.NewRequest(call.Method, reqPath, nil)
			var match mux.RouteMatch
			if !router.Match(req, &match) || match.MatchErr != nil || match.Route == nil {
				continue
			}
			tmpl, err := match.Route.GetPathTemplate()
			if err != nil {
				return nil, fmt.Errorf("%s %s matched a route without a template: %w", call.Method, call.Template, err)
			}
			if want != nil && match.Route == want {
				continue
			}
			if want == nil && normalizeRouteTemplate(tmpl) == call.Template {
				continue
			}
			if matched[tmpl] {
				continue
			}
			matched[tmpl] = true
			out = append(out, routeContractPrec{
				Method:   call.Method,
				Request:  call.Template,
				Expected: call.Template,
				Matched:  tmpl,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Request != out[j].Request {
			return out[i].Request < out[j].Request
		}
		if out[i].Method != out[j].Method {
			return out[i].Method < out[j].Method
		}
		return out[i].Matched < out[j].Matched
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
	code         bool // a token other than a static import declaration was read
	inImport     bool // the token just read was a static import declaration
	prevWord     string
	isClientFile bool
	api          map[string]bool
	ws           map[string]bool
	axios        map[string]bool
	instance     map[string]bool // axios instances bound in the client module
	bound        bool            // a binding for the next axios.create was recorded
	created      bool            // the client module's axios instance was created
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
		isClientFile: file == routeContractClientFile,
		api:          map[string]bool{},
		ws:           map[string]bool{},
		axios:        map[string]bool{},
		instance:     map[string]bool{},
	}
}

// bindsAxiosCreate reports whether the binding just read is followed by
// = <axios>.create(, and records that the next create call is bound. The
// position is unchanged.
func (s *routeScanner) bindsAxiosCreate() bool {
	mark := s.mark()
	defer s.restore(mark)
	s.skipSpaceComments()
	if !s.peek('=') || s.peekN(1) == '=' || s.peekN(1) == '>' {
		return false
	}
	s.i++
	s.skipSpaceComments()
	ident, _ := s.readIdent()
	if !s.axios[ident] || !s.consumeDot() {
		return false
	}
	if method, _ := s.readIdent(); method != "create" {
		return false
	}
	s.bound = true
	return true
}

// reExportSpec returns the module of an export ... from '<spec>' that follows
// the export keyword just read: export * [as x], export { ... }. A type-only
// re-export carries no runtime value and is not reported. The position is
// unchanged.
func (s *routeScanner) reExportSpec() (string, bool) {
	w := s.probe()
	w.skipSpaceComments()
	if w.peekIdent("type") {
		return "", false
	}
	switch {
	case w.peek('*'):
		w.i++
		w.skipSpaceComments()
		if w.consumeWord("as") {
			w.skipSpaceComments()
			if w.peek('\'') || w.peek('"') {
				if _, ok := w.readQuoted(); !ok {
					return "", false
				}
			} else if name, _ := w.readIdent(); name == "" {
				return "", false
			}
		}
	case w.peek('{'):
		for !w.eof() && !w.stop && !w.peek('}') {
			w.i++
			w.skipSpaceComments()
		}
		if w.eof() || w.stop {
			return "", false
		}
		w.i++
	default:
		return "", false
	}
	w.skipSpaceComments()
	if !w.consumeWord("from") {
		return "", false
	}
	w.skipSpaceComments()
	return w.readQuoted()
}

// probe returns a copy of the scanner at the same position for look-ahead.
// What it reads, records or reports is discarded; the main scan visits the
// same bytes afterwards.
func (s *routeScanner) probe() *routeScanner {
	return &routeScanner{
		file:         s.file,
		src:          s.src,
		i:            s.i,
		line:         s.line,
		canRegex:     s.canRegex,
		isClientFile: s.isClientFile,
		api:          map[string]bool{},
		ws:           map[string]bool{},
		axios:        map[string]bool{},
		instance:     map[string]bool{},
	}
}

// peekMember reports whether the next tokens are .<name>, without moving.
func (s *routeScanner) peekMember(name string) bool {
	mark := s.mark()
	defer s.restore(mark)
	if !s.consumeDot() {
		return false
	}
	s.skipSpaceComments()
	return s.peekIdent(name)
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
				s.i = lineCommentEnd(s.src, s.i)
				continue
			}
			if s.peekN(1) == '*' {
				s.skipBlockComment()
				continue
			}
			return
		default:
			if n := s.jsSpaceAt(s.i); n > 0 {
				s.i += n
				continue
			}
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
				i = lineCommentEnd(s.src, i)
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
			if n := s.jsSpaceAt(i); n > 0 {
				i += n
				continue
			}
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
	return unicode.IsLetter(r) || unicode.In(r, unicode.Nl, unicode.Other_ID_Start)
}

// isIdentCont is the byte-level test used when walking backwards, where a
// rune cannot be decoded; any non-ASCII byte counts, which only ever joins
// words and so fails closed. Forward reads use identContAt.
func (s *routeScanner) isIdentCont(c byte) bool {
	if c == '_' || c == '$' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
		return true
	}
	return c >= 0x80
}

// identContAt reports whether the rune at i continues an identifier. A
// non-ASCII rune must be one JavaScript allows there (ID_Continue, ZWNJ,
// ZWJ); whitespace such as U+00A0 ends the name instead of joining it.
func (s *routeScanner) identContAt(i int) bool {
	c := s.src[i]
	if c < 0x80 {
		return s.isIdentCont(c)
	}
	r, _ := utf8.DecodeRune(s.src[i:])
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.In(r, unicode.Nl, unicode.Mn, unicode.Mc, unicode.Pc, unicode.Other_ID_Start, unicode.Other_ID_Continue) || r == 0x200C || r == 0x200D
}

// jsSpaceAt returns the length of the non-ASCII JavaScript whitespace or line
// terminator at i (U+00A0, U+FEFF, other Zs, U+2028, U+2029), or 0.
func (s *routeScanner) jsSpaceAt(i int) int {
	if s.src[i] < 0x80 {
		return 0
	}
	r, size := utf8.DecodeRune(s.src[i:])
	if r == 0xFEFF || r == 0x2028 || r == 0x2029 || unicode.Is(unicode.Zs, r) {
		return size
	}
	return 0
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
	for !s.eof() && s.identContAt(s.i) {
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
	for i < len(s.src) && s.identContAt(i) {
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
		line := s.line
		if text, ok := s.readQuoted(); ok && isBrowserTransport(text) {
			s.fail(line, "unsupported string %q; it can name a browser HTTP transport", text)
		}
	case c == '`':
		line := s.line
		if text, ok := s.scanTemplate(false); ok && isBrowserTransport(text) {
			s.fail(line, "unsupported string %q; it can name a browser HTTP transport", text)
		}
	default:
		s.scanPunct(c)
	}
	if s.inImport {
		s.inImport = false
	} else {
		s.code = true
	}
}

// isBrowserTransport reports the global names that issue requests outside
// the api client.
func isBrowserTransport(name string) bool {
	switch name {
	case "fetch", "XMLHttpRequest", "WebSocket":
		return true
	default:
		return false
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
		if s.nextSignificant() == '(' {
			s.parseImport() // a dynamic import is an expression
			return
		}
		// Static imports are hoisted: a binding is in scope above its
		// declaration, where this single pass has not recorded it yet.
		if s.code {
			s.fail(line, "unsupported import after other code; move static imports to the top")
			return
		}
		s.parseImport()
		s.inImport = true
		return
	case "new":
		s.afterNew()
		return
	case "export":
		if spec, ok := s.reExportSpec(); ok && (isAPIClientModule(s.file, spec) || spec == "axios" || forbiddenHTTPModule(spec)) {
			// A barrel would hand out the client under a binding no
			// importer tracks; import it from the client module directly.
			s.fail(line, "unsupported re-export of %s", spec)
			return
		}
		if s.isClientFile && s.clientExport(line) {
			return
		}
		s.canRegex = true
		return
	case "function", "class", "interface", "enum", "namespace":
		s.canRegex = false
		return
	}
	if ident == "type" && s.looksLikeTypeAlias() {
		s.canRegex = false
		return
	}
	// Inside the client module the axios instance is bound as
	// const <name> = axios.create(...). Its supported uses there are
	// <name>.interceptors... and export default <name>; anything else could
	// issue a request without yielding a pair.
	if s.isClientFile && !member {
		if (prev == "const" || prev == "let" || prev == "var") && s.bindsAxiosCreate() {
			s.instance[ident] = true
			s.canRegex = false
			return
		}
		if s.instance[ident] {
			if !s.peekMember("interceptors") {
				s.fail(line, "unsupported use of the axios instance %q in the api client module", ident)
				return
			}
			s.canRegex = false
			return
		}
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
	// window.fetch, (window as any).fetch and friends are the same globals
	// reached through some receiver, which cannot be told apart from an
	// unrelated property. A member with such a name always fails; the
	// quoted form (window['fetch']) fails in scanOne.
	if member && isBrowserTransport(ident) {
		s.fail(line, "unsupported member .%s; HTTP calls go through the api client", ident)
		return
	}
	// fetch is never a supported transport. A call fails, and so does any
	// other reference (an alias, fetch.call, passing it on), which could reach
	// the server without yielding a pair. obj.fetch is a property.
	if ident == "fetch" && !member {
		mark := s.mark()
		s.skipSpaceComments()
		if s.peek('(') {
			s.fail(line, "unsupported call style fetch()")
			return
		}
		s.restore(mark)
		s.fail(line, "unsupported use of fetch; HTTP calls go through the api client")
		return
	}
	if (ident == "useWebSocket" || s.ws[ident]) && prev != "function" {
		if s.looksLikeCall() {
			s.finishTransport(line, "use-websocket")
			return
		}
		// An alias would open a socket the manifest never sees.
		if !member && prev != "typeof" {
			s.fail(line, "unsupported reference to %s; call it directly", ident)
			return
		}
	}
	if ident == "WebSocket" && s.looksLikeCall() {
		s.fail(line, "unsupported WebSocket() call without new")
		return
	}
	if ident == "WebSocket" && !member && !s.webSocketTypeUse(prev, s.i-len(ident)) {
		s.fail(line, "unsupported reference to WebSocket; open sockets with new WebSocket(...)")
		return
	}
	if (ident == "axios" || s.axios[ident]) && s.tryAxios(line, member) {
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

// clientExport checks an export in the api client module, just after export
// was read. Importers take its default export and its named export api as
// the client and pair their calls under the one checked baseURL, so every
// runtime export must be that instance. Only two shapes are accepted:
// export const <name> = axios.create(...) (bindsAxiosCreate and the create
// path check the rest) and export default <instance>, alone. Type-only
// exports carry no runtime value. Anything else fails: another declaration
// (let, a second declarator, async function, class ...), an export clause or
// star export (which can name the default as { default } from or a quoted
// "default"), or any other default export (a literal, an object, a function,
// api.interceptors..., a parenthesized form). It reports false, unchanged,
// for the accepted declaration and type-only exports.
func (s *routeScanner) clientExport(line int) bool {
	mark := s.mark()
	s.skipSpaceComments()
	if s.consumeWord("default") {
		s.skipSpaceComments()
		ident, _ := s.readIdent()
		if !s.instance[ident] || !s.endsStatement() {
			s.fail(line, "unsupported default export in the api client module; export default the axios instance")
			return true
		}
		s.canRegex = false
		s.prevWord = ""
		return true
	}
	decl := s.probe()
	decl.axios = s.axios
	switch kw, _ := decl.readIdent(); kw {
	case "type", "interface":
		s.restore(mark)
		return false
	case "const":
		decl.skipSpaceComments()
		if name, _ := decl.readIdent(); name != "" && decl.bindsAxiosCreate() {
			s.restore(mark)
			return false
		}
	}
	s.fail(line, "unsupported export in the api client module; export only const <name> = axios.create(...) and default <name>")
	return true
}

// endsStatement reports whether the expression just read ends its statement:
// the file ends, a ; or } follows, or the next token follows a line
// terminator (LF, CR, U+2028, U+2029, also inside a comment) and is a name
// that cannot continue an expression. Anything else (.get(...), a call, an
// operator, instanceof, in) continues it. The position is unchanged.
func (s *routeScanner) endsStatement() bool {
	mark := s.mark()
	defer s.restore(mark)
	start := s.i
	s.skipSpaceComments()
	if s.eof() || s.peek(';') || s.peek('}') {
		return true
	}
	gap := s.src[start:s.i]
	crossed := bytes.ContainsAny(gap, "\n\r") || bytes.Contains(gap, []byte("\u2028")) || bytes.Contains(gap, []byte("\u2029"))
	if !crossed || !s.isIdentStart() {
		return false
	}
	ident, _ := s.readIdent()
	switch ident {
	case "instanceof", "in": // TypeScript forbids a line break before as and satisfies
		return false
	}
	return true
}

// webSocketTypeUse reports the references to WebSocket, other than new
// WebSocket(...), that open nothing: its state constants, type operators
// and the two annotation forms the UI uses (let x: WebSocket and
// ): WebSocket {). Any other reference could be an alias that opens a socket
// the manifest never sees. start is where the name begins.
func (s *routeScanner) webSocketTypeUse(prev string, start int) bool {
	for _, state := range []string{"OPEN", "CONNECTING", "CLOSING", "CLOSED"} {
		if s.peekMember(state) {
			return true
		}
	}
	switch prev {
	case "typeof", "instanceof", "as", "satisfies":
		return true
	}
	back := func(i int) int { // the previous non-space byte before i, or -1
		for i--; i >= 0 && strings.ContainsRune(" \t\r\n", rune(s.src[i])); i-- {
		}
		return i
	}
	colon := back(start)
	if colon < 0 || s.src[colon] != ':' {
		return false
	}
	at := back(colon)
	if at < 0 {
		return false
	}
	if s.src[at] == ')' {
		return s.nextSignificant() == '{' // a return type before the body
	}
	end := at + 1
	for at >= 0 && s.isIdentCont(s.src[at]) {
		at--
	}
	if at+1 == end {
		return false
	}
	kw := back(at + 1)
	wordEnd := kw + 1
	for kw >= 0 && s.isIdentCont(s.src[kw]) {
		kw--
	}
	switch string(s.src[kw+1 : wordEnd]) {
	case "let", "const", "var":
		return true
	}
	return false
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
	if msg := s.checkCallConfig(method); msg != "" {
		s.fail(line, "%s", msg)
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

// checkCallConfig inspects the request config of the client call whose url
// argument was just read, and returns why it cannot be trusted, or "". A
// per-call baseURL sends the request somewhere other than routeContractAPIBase,
// so the config must be an object literal that provably sets none: no
// baseURL key, no spread and no computed key. The scan itself is unchanged;
// the main loop still visits every argument afterwards.
func (s *routeScanner) checkCallConfig(method string) string {
	args, ok := s.remainingArgs()
	if !ok {
		return fmt.Sprintf("unsupported arguments for %s()", method)
	}
	for _, arg := range args {
		if arg.spreadArg {
			// Which element lands in the config position is unknown.
			return fmt.Sprintf("unsupported spread argument for %s()", method)
		}
	}
	idx := 0 // get, delete, head, options: (url, config)
	switch method {
	case "post", "put", "patch": // (url, data, config)
		idx = 1
	}
	if idx >= len(args) {
		return ""
	}
	cfg := args[idx]
	switch {
	case !cfg.object:
		return fmt.Sprintf("unsupported request config for %s(); pass an object literal", method)
	case cfg.baseURL:
		return fmt.Sprintf("unsupported baseURL in the request config of %s()", method)
	case cfg.spread || cfg.computed:
		return fmt.Sprintf("unsupported spread or computed key in the request config of %s()", method)
	}
	return ""
}

type callArg struct {
	spreadArg  bool // the argument is spread (...xs) into several
	closed     bool // a bracket opened at the argument's top level just closed
	object     bool // the argument is an object literal and nothing else
	baseURL    bool // it names baseURL anywhere, as a word or a string
	baseURLKey bool // it has baseURL as a key at its own top level
	spread     bool // it spreads at its own top level
	computed   bool // it has a computed key at its own top level
}

// remainingArgs walks the arguments after the current one up to the call's
// closing parenthesis, on a copy of the scanner, so nothing is recorded and
// the position is unchanged. It reports false if the walk cannot finish.
func (s *routeScanner) remainingArgs() ([]callArg, bool) {
	args, _, ok := s.walkArgs(false)
	return args, ok
}

// callArgs walks every argument of the call whose ( comes next, like
// remainingArgs, and also returns the position just after its closing ).
// The scanner itself is unchanged.
func (s *routeScanner) callArgs() ([]callArg, scanMark, bool) {
	return s.walkArgs(true)
}

func (s *routeScanner) walkArgs(open bool) ([]callArg, scanMark, bool) {
	w := s.probe()
	var args []callArg
	var cur *callArg
	depth := 0
	keyPos := false // at an object literal key position of the current argument
	for {
		w.skipSpaceComments()
		if w.stop || w.eof() {
			return nil, scanMark{}, false
		}
		c := w.src[w.i]
		if open {
			// The opening parenthesis starts the first argument as a
			// comma starts the next one.
			if c != '(' {
				return nil, scanMark{}, false
			}
			open = false
			c = ','
		}
		if depth == 0 && (c == ')' || c == ',') {
			w.i++
			if c == ')' {
				return args, w.mark(), true
			}
			w.canRegex = true
			w.skipSpaceComments()
			if w.stop || w.eof() {
				return nil, scanMark{}, false
			}
			if w.peek(')') {
				w.i++
				return args, w.mark(), true // trailing comma
			}
			args = append(args, callArg{
				object:    w.peek('{'),
				spreadArg: w.peek('.') && w.peekN(1) == '.' && w.peekN(2) == '.',
			})
			cur = &args[len(args)-1]
			keyPos = false
			continue
		}
		if cur == nil {
			return nil, scanMark{}, false
		}
		if depth == 0 && cur.closed {
			// { ... }.key or { ... } ?? cfg: no longer the literal itself.
			cur.object = false
		}
		atKey := keyPos && depth == 1 && cur.object
		keyPos = false
		switch {
		case w.isIdentStart():
			ident, _ := w.readIdent()
			if ident == "baseURL" {
				cur.baseURL = true
				cur.baseURLKey = cur.baseURLKey || atKey
			}
			w.canRegex = unaryKeyword(ident)
			w.prevWord = ident
		case c >= '0' && c <= '9':
			w.scanNumber()
		case c == '\'' || c == '"':
			text, ok := w.readQuoted()
			if !ok {
				return nil, scanMark{}, false
			}
			if text == "baseURL" {
				cur.baseURL = true
			}
			w.canRegex = false
		case c == '`':
			if _, ok := w.scanTemplate(false); !ok {
				return nil, scanMark{}, false
			}
			w.canRegex = false
		case c == '(' || c == '[' || c == '{':
			if atKey && c == '[' {
				cur.computed = true
			}
			depth++
			w.scanPunct(c)
			keyPos = c == '{' && depth == 1
		case c == ')' || c == ']' || c == '}':
			depth--
			w.scanPunct(c)
			cur.closed = depth == 0
		case c == '.' && w.peekN(1) == '.' && w.peekN(2) == '.':
			if atKey {
				cur.spread = true
			}
			w.scanPunct(c)
		case c == ',':
			w.scanPunct(c)
			keyPos = depth == 1
		default:
			w.scanPunct(c)
		}
		w.member = false
	}
}

func (s *routeScanner) tryAxios(line int, member bool) bool {
	mark := s.mark()
	if !s.consumeDot() {
		// axios(config) / axios(url) reaches the server without a pair.
		if s.looksLikeCall() {
			s.fail(line, "unsupported axios() call")
			return true
		}
		// So does an alias (const request = axios) called later.
		if !member {
			s.fail(line, "unsupported reference to axios; only axios.create(...) is supported")
			return true
		}
		return false
	}
	method, _ := s.readIdent()
	if method == "create" && s.isClientFile {
		// Only an instance bound by bindsAxiosCreate is tracked.
		if !s.bound {
			s.fail(line, "unsupported axios.create(); bind it as const <name> = axios.create(...)")
			return true
		}
		s.bound = false
		// checkClientBase reads the one baseURL key; a spread or computed
		// key in the same options could replace it unseen.
		args, end, ok := s.callArgs()
		if !ok || len(args) != 1 || !args[0].object || args[0].spread || args[0].computed {
			s.fail(line, "unsupported axios.create() options; pass one object literal without spreads or computed keys")
			return true
		}
		// That baseURL describes one instance only: a second one, or one
		// whose options lack it, would be paired under a base it never uses.
		if s.created || !args[0].baseURLKey {
			s.fail(line, "unsupported axios.create(); the api client module creates one instance, with the checked baseURL")
			return true
		}
		// The binding holds the instance only if the call is the whole
		// initializer: axios.create(...).interceptors or && x would not.
		after := s.probe()
		after.restore(end)
		if !after.endsStatement() {
			s.fail(line, "unsupported axios.create() continuation; bind the instance as const <name> = axios.create(...)")
			return true
		}
		s.created = true
		s.canRegex = false
		s.prevWord = ""
		return true
	}
	if isHTTPMethod(method) || method == "request" {
		s.fail(line, "unsupported axios.%s call", method)
		return true
	}
	// In the client module only axios.create builds the checked instance;
	// any other member (Axios, defaults ...) could build or reach one unseen.
	if s.isClientFile && !member {
		s.fail(line, "unsupported axios.%s; only axios.create(...) is supported", method)
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
	if !isHTTPMethod(method) && (method != "request" || !s.isClientFile) {
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
	// The client module creates the axios instance under its own binding, so
	// any request it issues itself would bypass the import tracking: there it
	// fails whatever the argument.
	if s.isClientFile {
		s.fail(line, "unsupported http call .%s() in the api client module", method)
		return true
	}
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
			s.i = lineCommentEnd(s.src, s.i)
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
			b.WriteString(decoded)
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
			} else {
				if !s.scanInterp() {
					return "", false
				}
				// Outside path mode the text is only compared whole; an
				// interpolation keeps its neighbours apart.
				b.WriteByte(0)
			}
			continue
		}
		if c == '\n' {
			s.line++
		}
		b.WriteByte(c)
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
			// A named runtime import (Axios, create) builds an instance
			// the checked axios.create path never sees.
			if binding.imported != "default" {
				s.fail(s.line, "unsupported axios import %q in the api client module; import the default and call axios.create", binding.imported)
				return
			}
			s.axios[binding.local] = true
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
