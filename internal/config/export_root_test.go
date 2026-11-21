package config

import "testing"

func TestExportRootPrecedence(t *testing.T) {
	cases := []struct{ name, base, output, want string }{
		{"neither key set", "", "", ""},
		{"only export.output_directory", "", "/srv/out", "/srv/out"},
		{"only sync.export_base_dir", "/srv/base", "", "/srv/base"},
		{"both set: sync.export_base_dir wins", "/srv/base", "/srv/out", "/srv/base"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.Sync.ExportBaseDir = c.base
			cfg.Export.OutputDirectory = c.output
			if got := cfg.ExportRoot(); got != c.want {
				t.Fatalf("ExportRoot() = %q, want %q", got, c.want)
			}
		})
	}
}
