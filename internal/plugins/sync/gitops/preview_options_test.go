package gitops

import (
	"context"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/sync"
)

// The preview describes what an export with the same options would write: the common file and the
// templates directory appear only when their options are on.
func TestPreviewFollowsTheCommonAndTemplateOptions(t *testing.T) {
	data := &sync.ExportData{
		Devices:   []sync.DeviceData{{ID: 1, Name: "kitchen", Type: "shelly1"}},
		Templates: []sync.TemplateData{{ID: 1, Name: "base"}},
	}
	preview := func(common, templates bool) (string, int) {
		t.Helper()
		res, err := NewGitOpsExporter().Preview(context.Background(), data, sync.ExportConfig{Config: map[string]interface{}{
			"group_by":          "location",
			"include_common":    common,
			"include_templates": templates,
		}})
		if err != nil {
			t.Fatal(err)
		}
		return string(res.SampleData), res.RecordCount
	}

	onTree, onFiles := preview(true, true)
	offTree, offFiles := preview(false, false)

	// The root file is the one at the start of a line; each type directory has a common.yaml of its own.
	if !strings.Contains(onTree, "\n├── common.yaml") || !strings.Contains(onTree, "templates/") {
		t.Errorf("options on: the tree lacks the common file or the templates:\n%s", onTree)
	}
	if strings.Contains(offTree, "\n├── common.yaml") || strings.Contains(offTree, "templates/") {
		t.Errorf("options off: the tree still shows the common file or the templates:\n%s", offTree)
	}
	// common.yaml and one template are the two files the options add.
	if onFiles-offFiles != 2 {
		t.Errorf("file count: %d with the options, %d without; want a difference of 2", onFiles, offFiles)
	}
}

// A device that matches no group is written under ungrouped/<type>/, so the preview lists it there and
// counts one file per such device, whatever the number of types.
func TestPreviewListsUngroupedDevices(t *testing.T) {
	data := &sync.ExportData{Devices: []sync.DeviceData{
		{ID: 1, Name: "sensor-1", Type: "shelly1"},
		{ID: 2, Name: "sensor-2", Type: "shelly1"},
		{ID: 3, Name: "plug-1", Type: "shellyplug"},
	}}
	res, err := NewGitOpsExporter().Preview(context.Background(), data, sync.ExportConfig{Config: map[string]interface{}{
		"group_by":          "location",
		"include_common":    false,
		"include_templates": false,
	}})
	if err != nil {
		t.Fatal(err)
	}
	tree := string(res.SampleData)
	for _, want := range []string{"ungrouped/", "sensor-1.yaml", "sensor-2.yaml", "plug-1.yaml"} {
		if !strings.Contains(tree, want) {
			t.Errorf("the tree lacks %q:\n%s", want, tree)
		}
	}
	// Three device files and export-summary.yaml.
	if res.RecordCount != 4 {
		t.Errorf("file count = %d, want 4", res.RecordCount)
	}
}
