package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/version"
)

// runMainEnv makes the test binary run the real main() on the arguments after it, as the shipped binary does.
const runMainEnv = "SHELLY_TEST_RUN_MAIN"

// --version prints the version the binary was stamped with and loads no configuration. The test runs
// main(), which installs the initialiser, in a child process with a config path that does not exist:
// initApp would stop the child on it, so a clean exit with the version printed means cobra answered
// --version before the initialisers ran.
func TestRootVersionFlag(t *testing.T) {
	if os.Getenv(runMainEnv) == "1" {
		os.Args = []string{"shelly-manager", "--config", os.Getenv(runMainEnv + "_CONFIG"), "--version"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRootVersionFlag$")
	cmd.Env = append(os.Environ(), runMainEnv+"=1", runMainEnv+"_CONFIG="+filepath.Join(t.TempDir(), "absent.yaml"))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.True(t, strings.HasPrefix(string(out), "shelly-manager version "+version.Version+"\n"), string(out))
}
