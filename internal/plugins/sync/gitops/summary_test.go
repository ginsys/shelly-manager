package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// The summary file says what was exported. It must not copy the request configuration: that holds
// whatever the caller sent, and the output directory usually goes into a git repository.
func TestGitOpsSummaryCarriesNoRequestConfig(t *testing.T) {
	dir := t.TempDir()

	exporter := newTestExporter()
	if err := exporter.Initialize(logging.GetDefault()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	secrets := []string{"tok-top-level-4711", "hunter2-nested", "https://user:urlpass-9@example.invalid/hook"}
	data := &sync.ExportData{
		Devices:  []sync.DeviceData{{ID: 1, Name: "Kitchen Switch", Type: "SHSW-1", MAC: "00:11:22:33:44:66"}},
		Metadata: sync.ExportMetadata{ExportID: "summary-test", SystemVersion: "1.0.0", TotalDevices: 1},
	}
	config := sync.ExportConfig{
		Format: "yaml",
		Config: map[string]interface{}{
			"output_path": dir,
			"group_by":    "custom",
			"api_token":   secrets[0],
			"group_mapping": map[string]interface{}{
				"auth": map[string]interface{}{"password": secrets[1]},
			},
			"webhook": secrets[2],
		},
	}

	if _, err := exporter.Export(context.Background(), data, config); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// No file the export wrote holds any of the three values.
	files := 0
	walkErr := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		files++
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, secret := range secrets {
			if strings.Contains(string(content), secret) {
				t.Errorf("%s holds the request secret %q", path, secret)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}
	if files == 0 {
		t.Fatal("the walk read no file, so the check above proves nothing")
	}

	// The summary is still written, with its three parts and no config entry.
	raw, err := os.ReadFile(filepath.Join(dir, "export-summary.yaml"))
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	var summary map[string]interface{}
	if err := yaml.Unmarshal(raw, &summary); err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	for _, key := range []string{"export_metadata", "structure", "generated_at"} {
		if _, ok := summary[key]; !ok {
			t.Errorf("summary lost %q", key)
		}
	}
	if _, ok := summary["config"]; ok {
		t.Error("summary still has a config entry")
	}
}
