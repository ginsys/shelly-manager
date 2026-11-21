package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// The schema is the contract: include_common and include_templates default to true, so a client that
// leaves them out gets the common file and the templates, and one that sends false gets neither.
func includeDefaultsData() *sync.ExportData {
	return &sync.ExportData{
		Devices:   []sync.DeviceData{{ID: 1, Name: "kitchen", Type: "shelly1"}},
		Templates: []sync.TemplateData{{ID: 1, Name: "base"}},
		Metadata:  sync.ExportMetadata{ExportID: "include-defaults", SystemVersion: "1.0.0", TotalDevices: 1},
	}
}

func TestExportReadsAMissingIncludeKeyAsTheSchemaDefault(t *testing.T) {
	cases := []struct {
		name      string
		options   map[string]interface{}
		wantFiles bool
	}{
		{"keys omitted", map[string]interface{}{}, true},
		{"keys false", map[string]interface{}{"include_common": false, "include_templates": false}, false},
		{"keys true", map[string]interface{}{"include_common": true, "include_templates": true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			exporter := NewGitOpsExporter()
			if err := exporter.Initialize(logging.GetDefault()); err != nil {
				t.Fatalf("Initialize: %v", err)
			}
			cfg := map[string]interface{}{"output_path": dir, "group_by": "location"}
			for k, v := range tc.options {
				cfg[k] = v
			}
			if _, err := exporter.Export(context.Background(), includeDefaultsData(), sync.ExportConfig{Format: "yaml", Config: cfg}); err != nil {
				t.Fatalf("Export: %v", err)
			}
			_, commonErr := os.Stat(filepath.Join(dir, "common.yaml"))
			_, templatesErr := os.Stat(filepath.Join(dir, "templates"))
			if gotCommon := commonErr == nil; gotCommon != tc.wantFiles {
				t.Errorf("common.yaml present = %v, want %v", gotCommon, tc.wantFiles)
			}
			if gotTemplates := templatesErr == nil; gotTemplates != tc.wantFiles {
				t.Errorf("templates/ present = %v, want %v", gotTemplates, tc.wantFiles)
			}
		})
	}
}

func TestPreviewReadsAMissingIncludeKeyAsTheSchemaDefault(t *testing.T) {
	preview := func(options map[string]interface{}) string {
		t.Helper()
		cfg := map[string]interface{}{"group_by": "location"}
		for k, v := range options {
			cfg[k] = v
		}
		res, err := NewGitOpsExporter().Preview(context.Background(), includeDefaultsData(), sync.ExportConfig{Config: cfg})
		if err != nil {
			t.Fatal(err)
		}
		return string(res.SampleData)
	}
	listsBoth := func(tree string) bool {
		return strings.Contains(tree, "\n├── common.yaml") && strings.Contains(tree, "templates/")
	}

	if tree := preview(nil); !listsBoth(tree) {
		t.Errorf("keys omitted: the preview lacks the common file or the templates:\n%s", tree)
	}
	off := preview(map[string]interface{}{"include_common": false, "include_templates": false})
	if strings.Contains(off, "\n├── common.yaml") || strings.Contains(off, "templates/") {
		t.Errorf("keys false: the preview still lists the common file or the templates:\n%s", off)
	}
}

// A present value of the wrong type is refused, as a wrong group_by or output_path is.
func TestValidateConfigRefusesANonBooleanInclude(t *testing.T) {
	exporter := NewGitOpsExporter()
	for _, key := range []string{"include_common", "include_templates"} {
		err := exporter.ValidateConfig(map[string]interface{}{key: "yes"})
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s = \"yes\": got %v, want a refusal naming the key", key, err)
		}
		if err := exporter.ValidateConfig(map[string]interface{}{key: false}); err != nil {
			t.Errorf("%s = false: got %v, want accepted", key, err)
		}
	}
	if err := exporter.ValidateConfig(map[string]interface{}{}); err != nil {
		t.Errorf("keys omitted: got %v, want accepted", err)
	}
}
