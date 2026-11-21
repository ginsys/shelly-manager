package config

// ExportRoot returns the directory exports are written under and downloaded from: sync.export_base_dir when
// set, else export.output_directory when set. Empty means neither is set, and the sync package applies its
// default root (sync.DefaultExportRoot).
func (c *Config) ExportRoot() string {
	if c.Sync.ExportBaseDir != "" {
		return c.Sync.ExportBaseDir
	}
	return c.Export.OutputDirectory
}
