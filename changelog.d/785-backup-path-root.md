Changed

- A backup is now written only inside one directory: `sync.export_base_dir` when it is set, `./data/backups` when it is not. A backup export whose `output_path` is outside it answers `400` with that directory in the reason and creates nothing; before, it wrote to any directory the server could write. An empty `output_path` still means the default, and the result's `output_path` is now an absolute path. A deployment that kept backups elsewhere sets `sync.export_base_dir` (and `export.output_directory`, for downloads) to that directory. Fixes ginsys/shelly-manager#785.
