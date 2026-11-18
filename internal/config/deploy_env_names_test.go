package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// The app reads only SHELLY_ + a config key with "." as "_" (Load). The
// Kubernetes manifests once set unprefixed names the app never read, so every
// deploy from them ran the baked SQLite config (#571). These tests read the
// kustomize files and hold every environment name in them to that rule.

const kustomizeDir = "../../deploy/kubernetes/kustomize"

// secretEnvNames may reach the pod only through a secretKeyRef, never as a
// ConfigMap or literal value.
var secretEnvNames = map[string]bool{
	"SHELLY_DATABASE_USERNAME":                 true,
	"SHELLY_DATABASE_PASSWORD":                 true,
	"SHELLY_NOTIFICATIONS_EMAIL_SMTP_PASSWORD": true,
	"SHELLY_OPNSENSE_API_KEY":                  true,
	"SHELLY_OPNSENSE_API_SECRET":               true,
	"SHELLY_SECURITY_ADMIN_API_KEY":            true,
	"SHELLY_API_KEY":                           true,
}

type envEntry struct {
	Source     string // file and place, for the failure message
	Name       string
	Value      string
	FromSecret bool // a secretKeyRef entry
}

func readKustomizeYAML(t *testing.T, rel string, into func(*yaml.Decoder) error) {
	t.Helper()
	f, err := os.Open(filepath.Join(kustomizeDir, rel))
	if err != nil {
		t.Fatalf("open %s: %v", rel, err)
	}
	defer func() { _ = f.Close() }()
	if err := into(yaml.NewDecoder(f)); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
}

// configMapEntries returns the data keys of every ConfigMap in a file.
func configMapEntries(t *testing.T, rel string) []envEntry {
	t.Helper()
	var out []envEntry
	readKustomizeYAML(t, rel, func(d *yaml.Decoder) error {
		for {
			var doc struct {
				Kind string            `yaml:"kind"`
				Data map[string]string `yaml:"data"`
			}
			if err := d.Decode(&doc); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return err
			}
			if doc.Kind != "ConfigMap" {
				continue
			}
			for k, v := range doc.Data {
				out = append(out, envEntry{Source: rel + " data", Name: k, Value: v})
			}
		}
	})
	return out
}

// containerEntries returns the env entries of every container in the
// Deployment and DaemonSet documents of a file (envFrom is the ConfigMap).
func containerEntries(t *testing.T, rel string) []envEntry {
	t.Helper()
	var out []envEntry
	readKustomizeYAML(t, rel, func(d *yaml.Decoder) error {
		for {
			var doc struct {
				Kind string `yaml:"kind"`
				Spec struct {
					Template struct {
						Spec struct {
							Containers []struct {
								Name string `yaml:"name"`
								Env  []struct {
									Name      string         `yaml:"name"`
									Value     string         `yaml:"value"`
									ValueFrom map[string]any `yaml:"valueFrom"`
								} `yaml:"env"`
							} `yaml:"containers"`
						} `yaml:"spec"`
					} `yaml:"template"`
				} `yaml:"spec"`
			}
			if err := d.Decode(&doc); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return err
			}
			for _, c := range doc.Spec.Template.Spec.Containers {
				for _, e := range c.Env {
					_, fromSecret := e.ValueFrom["secretKeyRef"]
					out = append(out, envEntry{
						Source: rel + " " + doc.Kind + "/" + c.Name, Name: e.Name,
						Value: e.Value, FromSecret: fromSecret,
					})
				}
			}
		}
	})
	return out
}

// generatorLiterals returns the literals of the configMapGenerator entries in one
// kustomization, from the generator called only, or from every generator when only is "".
func generatorLiterals(d *yaml.Decoder, rel, only string) ([]envEntry, error) {
	var doc struct {
		Generators []struct {
			Name     string   `yaml:"name"`
			Literals []string `yaml:"literals"`
		} `yaml:"configMapGenerator"`
	}
	if err := d.Decode(&doc); err != nil {
		return nil, err
	}
	var out []envEntry
	for _, g := range doc.Generators {
		if only != "" && g.Name != only {
			continue
		}
		for _, lit := range g.Literals {
			k, v, _ := strings.Cut(lit, "=")
			out = append(out, envEntry{Source: rel + " literal", Name: k, Value: v})
		}
	}
	return out, nil
}

// generatorEntries returns the configMapGenerator literals of an overlay.
func generatorEntries(t *testing.T, rel string) []envEntry {
	return readGenerator(t, rel, "")
}

// managerConfigEntries returns only the literals that land in the manager's ConfigMap.
func managerConfigEntries(t *testing.T, rel string) []envEntry {
	return readGenerator(t, rel, managerConfigMap)
}

func readGenerator(t *testing.T, rel, only string) []envEntry {
	t.Helper()
	var out []envEntry
	readKustomizeYAML(t, rel, func(d *yaml.Decoder) (err error) {
		out, err = generatorLiterals(d, rel, only)
		return err
	})
	return out
}

// readableEnvNames is every environment name the app reads: SHELLY_ + each key
// viper knows after Load, with "." as "_".
func readableEnvNames(t *testing.T) map[string]bool {
	t.Helper()
	if _, err := Load("../../configs/shelly-manager.yaml"); err != nil {
		t.Fatalf("Load example config: %v", err)
	}
	names := map[string]bool{}
	for _, key := range viper.AllKeys() {
		names["SHELLY_"+strings.ToUpper(strings.ReplaceAll(key, ".", "_"))] = true
	}
	return names
}

// urlCarriesCredentials reports userinfo, or any query or fragment. pgx reads
// user, password, passfile and service from the query under spellings net/url
// does not agree with (padded or encoded keys, a '#'), so a manifest URL carries
// no query at all: connection options belong in the app's own settings.
func urlCarriesCredentials(raw string) bool {
	if strings.ContainsAny(raw, "?#") {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.User != nil
}

// problems lists what is wrong with a set of env entries; nil means none.
func problems(entries []envEntry, readable map[string]bool) []string {
	var out []string
	for _, e := range entries {
		if !readable[e.Name] {
			out = append(out, e.Source+": "+e.Name+" is not a name the app reads (SHELLY_ + config key)")
		}
		if secretEnvNames[e.Name] && !e.FromSecret {
			out = append(out, e.Source+": "+e.Name+" is a credential and must come from a secretKeyRef")
		}
		if strings.Contains(e.Value, "://") {
			if urlCarriesCredentials(e.Value) {
				out = append(out, e.Source+": "+e.Name+" carries credentials, or a query that could, in its URL")
			}
		}
	}
	sort.Strings(out)
	return out
}

func kustomizeEnvEntries(t *testing.T) []envEntry {
	t.Helper()
	entries := configMapEntries(t, "base/configmap.yaml")
	entries = append(entries, containerEntries(t, "base/deployment.yaml")...)
	entries = append(entries, configMapEntries(t, "overlays/development/configmap-dev.yaml")...)
	for _, overlay := range []string{"development", "staging", "production"} {
		entries = append(entries, generatorEntries(t, "overlays/"+overlay+"/kustomization.yaml")...)
	}
	return entries
}

func TestDeployEnvNames(t *testing.T) {
	readable := readableEnvNames(t)
	entries := kustomizeEnvEntries(t)
	if len(entries) < 50 {
		t.Fatalf("read only %d env entries from the kustomize tree; the readers are broken", len(entries))
	}

	for _, p := range problems(entries, readable) {
		t.Error(p)
	}
}

// The checker is only worth running if it fails on the mistakes it is for.
func TestDeployEnvNames_CheckerFindsTheMistakes(t *testing.T) {
	readable := readableEnvNames(t)

	bad := []envEntry{
		{Source: "a", Name: "DATABASE_PROVIDER", Value: "postgresql"},
		{Source: "b", Name: "LOG_LEVEL", Value: "info"},
		{Source: "c", Name: "SHELLY_DATABASE_PASSWORD", Value: "hunter2"},
		{Source: "d", Name: "SHELLY_DATABASE_USERNAME", Value: "shelly"},
		{Source: "e", Name: "SHELLY_DATABASE_DSN", Value: "postgresql://shelly:hunter2@db/shelly"},
		{Source: "i", Name: "SHELLY_DATABASE_DSN", Value: "postgresql://db/shelly?user=app&password=hunter2"},
		{Source: "j", Name: "SHELLY_DATABASE_DSN", Value: "postgresql://db/shelly?pass%77ord=hunter2"},
		{Source: "k", Name: "SHELLY_DATABASE_DSN", Value: "postgresql://db/shelly? password =hunter2"},
		{Source: "l", Name: "SHELLY_DATABASE_DSN", Value: "postgresql://db/shelly?application_name=x#&password=hunter2"},
	}
	if got := problems(bad, readable); len(got) != 9 {
		t.Errorf("found %d problems in 9 mistakes: %q", len(got), got)
	}

	good := []envEntry{
		{Source: "f", Name: "SHELLY_LOGGING_LEVEL", Value: "info"},
		{Source: "g", Name: "SHELLY_DATABASE_PASSWORD", FromSecret: true},
		{Source: "h", Name: "SHELLY_DATABASE_DSN", Value: "postgresql://db:5432/shelly"},
	}
	if got := problems(good, readable); got != nil {
		t.Errorf("flagged correct entries: %q", got)
	}
}

// What a pod gets from an overlay, loaded the way the app loads it: the base
// ConfigMap, the overlay's literals and patch, and the credentials the Secret
// supplies. The DSN stays credential-less; the app adds them.
func TestDeployEnvNames_OverlayReachesTheApp(t *testing.T) {
	const user, password = "shelly", "p@ss:w/rd%41;--x=y"

	overlayEnv := func(overlay string) map[string]string {
		env := map[string]string{}
		entries := configMapEntries(t, "base/configmap.yaml")
		if overlay == "development" {
			entries = append(entries, configMapEntries(t, "overlays/development/configmap-dev.yaml")...)
		}
		entries = append(entries, generatorEntries(t, "overlays/"+overlay+"/kustomization.yaml")...)
		for _, e := range entries {
			env[e.Name] = e.Value
		}
		return env
	}

	for _, overlay := range []string{"staging", "production"} {
		t.Run(overlay, func(t *testing.T) {
			for k, v := range overlayEnv(overlay) {
				t.Setenv(k, v)
			}
			t.Setenv("SHELLY_DATABASE_USERNAME", user)
			t.Setenv("SHELLY_DATABASE_PASSWORD", password)

			cfg, err := Load("../../configs/shelly-manager.yaml")
			if err != nil {
				t.Fatal(err)
			}
			got := cfg.GetDatabaseConfig()

			if got.Provider != "postgresql" {
				t.Errorf("provider = %q, want postgresql", got.Provider)
			}
			if got.DSN != "postgresql://postgresql:5432/shelly" {
				t.Errorf("dsn = %q", got.DSN)
			}
			if got.Username != user || got.Password != password {
				t.Errorf("credentials did not reach the database config: %q / %q", got.Username, got.Password)
			}
			if got.MaxOpenConns != 8 || got.MaxIdleConns != 2 {
				t.Errorf("pool = %d open / %d idle, want 8 / 2", got.MaxOpenConns, got.MaxIdleConns)
			}
			if cfg.Logging.Level != "info" || cfg.Logging.Format != "json" {
				t.Errorf("logging = %q / %q, want info / json", cfg.Logging.Level, cfg.Logging.Format)
			}
			if !reflect.DeepEqual(cfg.Discovery.Networks, []string{"192.168.1.0/24"}) {
				t.Errorf("discovery.networks = %v", cfg.Discovery.Networks)
			}
			if cfg.Provisioning.ProvisionInterval != 600 || cfg.Provisioning.AutoProvision {
				t.Errorf("provisioning = %d / %v", cfg.Provisioning.ProvisionInterval, cfg.Provisioning.AutoProvision)
			}
		})
	}

	t.Run("development stays on SQLite", func(t *testing.T) {
		for k, v := range overlayEnv("development") {
			t.Setenv(k, v)
		}

		cfg, err := Load("../../configs/shelly-manager.yaml")
		if err != nil {
			t.Fatal(err)
		}
		got := cfg.GetDatabaseConfig()

		if got.Provider != "sqlite" || got.DSN != "/tmp/shelly.db" {
			t.Errorf("database = %q %q, want sqlite /tmp/shelly.db", got.Provider, got.DSN)
		}
		if cfg.Logging.Level != "debug" || cfg.Logging.Format != "text" {
			t.Errorf("logging = %q / %q, want debug / text", cfg.Logging.Level, cfg.Logging.Format)
		}
	})
}

// The provisioner runs on the host network, so it reaches the manager only through a URL that
// names the overlay's rendered Service and port, and only if cluster DNS applies to a host-network
// pod (#591). The expected URL is derived from the files that render the Service: the overlay's
// namePrefix, the base Service's name and its http port.

const (
	provisionerURLName = "SHELLY_API_URL"
	managerConfigMap   = "shelly-manager-config"
)

// provisionerFacts is what the base DaemonSet says about how the provisioner finds the manager.
type provisionerFacts struct {
	HostNetwork bool
	DNSPolicy   string
	URLRef      string // "<configmap>/<key>" that SHELLY_API_URL is read from; empty when it is not
}

// provisionerDaemonSet reads the DaemonSet of the base manifests.
func provisionerDaemonSet(t *testing.T) provisionerFacts {
	t.Helper()
	var out provisionerFacts
	found := false
	readKustomizeYAML(t, "base/deployment.yaml", func(d *yaml.Decoder) error {
		for {
			var doc struct {
				Kind string `yaml:"kind"`
				Spec struct {
					Template struct {
						Spec struct {
							HostNetwork bool   `yaml:"hostNetwork"`
							DNSPolicy   string `yaml:"dnsPolicy"`
							Containers  []struct {
								Name string `yaml:"name"`
								Env  []struct {
									Name      string `yaml:"name"`
									ValueFrom struct {
										ConfigMapKeyRef struct {
											Name string `yaml:"name"`
											Key  string `yaml:"key"`
										} `yaml:"configMapKeyRef"`
									} `yaml:"valueFrom"`
								} `yaml:"env"`
							} `yaml:"containers"`
						} `yaml:"spec"`
					} `yaml:"template"`
				} `yaml:"spec"`
			}
			if err := d.Decode(&doc); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return err
			}
			if doc.Kind != "DaemonSet" {
				continue
			}
			spec := doc.Spec.Template.Spec
			for _, c := range spec.Containers {
				if c.Name != "provisioner" {
					continue
				}
				found = true
				out = provisionerFacts{HostNetwork: spec.HostNetwork, DNSPolicy: spec.DNSPolicy}
				for _, e := range c.Env {
					if ref := e.ValueFrom.ConfigMapKeyRef; e.Name == provisionerURLName && ref.Name != "" {
						out.URLRef = ref.Name + "/" + ref.Key
					}
				}
			}
		}
	})
	if !found {
		t.Fatal("base/deployment.yaml has no DaemonSet container named provisioner; the reader is broken")
	}
	return out
}

// managerService returns the name and the http port of the base manager Service.
func managerService(t *testing.T) (string, int) {
	t.Helper()
	const name = "shelly-manager"
	port := 0
	readKustomizeYAML(t, "base/service.yaml", func(d *yaml.Decoder) error {
		for {
			var doc struct {
				Kind     string `yaml:"kind"`
				Metadata struct {
					Name string `yaml:"name"`
				} `yaml:"metadata"`
				Spec struct {
					Ports []struct {
						Name string `yaml:"name"`
						Port int    `yaml:"port"`
					} `yaml:"ports"`
				} `yaml:"spec"`
			}
			if err := d.Decode(&doc); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return err
			}
			if doc.Kind != "Service" || doc.Metadata.Name != name {
				continue
			}
			for _, p := range doc.Spec.Ports {
				if p.Name == "http" {
					port = p.Port
				}
			}
		}
	})
	if port == 0 {
		t.Fatalf("base/service.yaml has no Service %s with a port named http; the reader is broken", name)
	}
	return name, port
}

// overlayPrefix returns the namePrefix of an overlay's kustomization.
func overlayPrefix(t *testing.T, rel string) string {
	t.Helper()
	var doc struct {
		NamePrefix string `yaml:"namePrefix"`
	}
	readKustomizeYAML(t, rel, func(d *yaml.Decoder) error { return d.Decode(&doc) })
	return doc.NamePrefix
}

// overlayNames lists the directories under overlays/, so a new overlay is checked without an edit here.
func overlayNames(t *testing.T) []string {
	t.Helper()
	dirs, err := os.ReadDir(filepath.Join(kustomizeDir, "overlays"))
	if err != nil {
		t.Fatalf("read overlays: %v", err)
	}
	var out []string
	for _, d := range dirs {
		if d.IsDir() {
			out = append(out, d.Name())
		}
	}
	return out
}

// provisionerURLProblems lists what keeps the provisioner of one overlay from reaching the manager.
func provisionerURLProblems(source, prefix, service string, port int, literals []envEntry, ds provisionerFacts) []string {
	var out []string
	want := fmt.Sprintf("http://%s%s:%d", prefix, service, port)
	got, set := "", false
	for _, e := range literals {
		if e.Name == provisionerURLName {
			got, set = e.Value, true
		}
	}
	switch {
	case !set:
		out = append(out, source+": sets no "+provisionerURLName+" in "+managerConfigMap+", so the provisioner's configMapKeyRef has no key to read and the pod does not start")
	case got != want:
		out = append(out, fmt.Sprintf("%s: %s = %q, want %q (the rendered Service and its port)", source, provisionerURLName, got, want))
	}
	if wantRef := managerConfigMap + "/" + provisionerURLName; ds.URLRef != wantRef {
		out = append(out, fmt.Sprintf("base/deployment.yaml: the provisioner reads %s from %q, want a configMapKeyRef %q", provisionerURLName, ds.URLRef, wantRef))
	}
	if ds.HostNetwork && ds.DNSPolicy != "ClusterFirstWithHostNet" {
		out = append(out, fmt.Sprintf("base/deployment.yaml: the provisioner is on the host network with dnsPolicy %q, so a Service name does not resolve; want ClusterFirstWithHostNet", ds.DNSPolicy))
	}
	return out
}

func TestDeployEnvNames_ProvisionerReachesTheManager(t *testing.T) {
	ds := provisionerDaemonSet(t)
	service, port := managerService(t)
	overlays := overlayNames(t)
	if len(overlays) < 3 {
		t.Fatalf("read %d overlays, want development, staging and production; the reader is broken", len(overlays))
	}
	for _, overlay := range overlays {
		rel := "overlays/" + overlay + "/kustomization.yaml"
		for _, p := range provisionerURLProblems(rel, overlayPrefix(t, rel), service, port, managerConfigEntries(t, rel), ds) {
			t.Error(p)
		}
	}
}

// The checker is only worth running if it fails on the mistakes it is for.
func TestDeployEnvNames_ProvisionerCheckerFindsTheMistakes(t *testing.T) {
	good := provisionerFacts{HostNetwork: true, DNSPolicy: "ClusterFirstWithHostNet", URLRef: managerConfigMap + "/" + provisionerURLName}
	url := func(v string) []envEntry { return []envEntry{{Name: provisionerURLName, Value: v}} }

	if got := provisionerURLProblems("o", "dev-", "shelly-manager", 80, url("http://dev-shelly-manager:80"), good); got != nil {
		t.Errorf("flagged a correct overlay: %q", got)
	}

	// The URL in some other generator never reaches the ConfigMap the pod reads.
	const otherGenerator = "configMapGenerator:\n" +
		"- name: " + managerConfigMap + "\n  literals:\n  - A=b\n" +
		"- name: unrelated-config\n  literals:\n  - " + provisionerURLName + "=http://dev-shelly-manager:80\n"
	elsewhere, err := generatorLiterals(yaml.NewDecoder(strings.NewReader(otherGenerator)), "o", managerConfigMap)
	if err != nil {
		t.Fatalf("parse the fixture: %v", err)
	}
	// Control: the unfiltered read does see the URL, so the filter is what hides it.
	if all, _ := generatorLiterals(yaml.NewDecoder(strings.NewReader(otherGenerator)), "o", ""); len(all) != 2 {
		t.Fatalf("the unfiltered read found %d literals, want 2; the fixture is broken", len(all))
	}

	cases := []struct {
		name     string
		literals []envEntry
		ds       provisionerFacts
	}{
		{"no literal", nil, good},
		{"url in another generator", elsewhere, good},
		{"url key of another ConfigMap", url("http://dev-shelly-manager:80"), provisionerFacts{HostNetwork: true, DNSPolicy: good.DNSPolicy, URLRef: "other-config/" + provisionerURLName}},
		{"url key of another name", url("http://dev-shelly-manager:80"), provisionerFacts{HostNetwork: true, DNSPolicy: good.DNSPolicy, URLRef: managerConfigMap + "/SHELLY_LOGGING_LEVEL"}},
		{"another dnsPolicy", url("http://dev-shelly-manager:80"), provisionerFacts{HostNetwork: true, DNSPolicy: "ClusterFirst", URLRef: good.URLRef}},
		{"prefix of another overlay", url("http://prod-shelly-manager:80"), good},
		{"container port instead of the Service port", url("http://dev-shelly-manager:8080"), good},
		{"no prefix", url("http://shelly-manager:80"), good},
		{"no dnsPolicy on the host network", url("http://dev-shelly-manager:80"), provisionerFacts{HostNetwork: true, URLRef: good.URLRef}},
		{"url not read from the ConfigMap", url("http://dev-shelly-manager:80"), provisionerFacts{HostNetwork: true, DNSPolicy: good.DNSPolicy}},
	}
	for _, c := range cases {
		if got := provisionerURLProblems("o", "dev-", "shelly-manager", 80, c.literals, c.ds); len(got) != 1 {
			t.Errorf("%s: found %d problems, want 1: %q", c.name, len(got), got)
		}
	}
}
