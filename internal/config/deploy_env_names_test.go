package config

import (
	"errors"
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

// generatorEntries returns the configMapGenerator literals of an overlay.
func generatorEntries(t *testing.T, rel string) []envEntry {
	t.Helper()
	var out []envEntry
	readKustomizeYAML(t, rel, func(d *yaml.Decoder) error {
		var doc struct {
			Generators []struct {
				Literals []string `yaml:"literals"`
			} `yaml:"configMapGenerator"`
		}
		if err := d.Decode(&doc); err != nil {
			return err
		}
		for _, g := range doc.Generators {
			for _, lit := range g.Literals {
				k, v, _ := strings.Cut(lit, "=")
				out = append(out, envEntry{Source: rel + " literal", Name: k, Value: v})
			}
		}
		return nil
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

// urlCarriesCredentials reports userinfo, or a user/password/sslpassword query
// parameter: pgx applies those after the userinfo, so they would override the
// credentials the Secret supplies. Query keys are compared after decoding.
func urlCarriesCredentials(u *url.URL) bool {
	if u.User != nil {
		return true
	}
	for key := range u.Query() {
		switch strings.ToLower(key) {
		case "user", "password", "sslpassword":
			return true
		}
	}
	return false
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
			if u, err := url.Parse(e.Value); err == nil && urlCarriesCredentials(u) {
				out = append(out, e.Source+": "+e.Name+" carries credentials in its URL")
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
	}
	if got := problems(bad, readable); len(got) != 7 {
		t.Errorf("found %d problems in 7 mistakes: %q", len(got), got)
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
