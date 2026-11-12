package database

import (
	"github.com/ginsys/shelly-manager/internal/configuration"
)

// ConfigTemplate is a reusable configuration template. The model lives in the
// configuration package, which this package imports and which therefore cannot
// import this one; the alias keeps the Manager's template methods and the schema
// fixups on the one owner of the table.
type ConfigTemplate = configuration.ConfigTemplate
