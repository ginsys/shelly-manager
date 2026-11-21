Fixed

- The JSON and YAML exports now write their file (plain, gzip or zip) through the opened export directory (`os.Root`), as the SMA and GitOps exports do, so a symbolic link swapped in between the path check and the write can no longer send the write outside the export directory. A link whose target is an absolute path is now refused there too, even one that points inside the directory. The backup file is still written by path. Refs ginsys/shelly-manager#815.
