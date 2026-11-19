package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The agent has no manager to talk to until it is told one. With no URL in the flag, the config file or
// SHELLY_API_URL it must stop and say so, and must not fall back to an address that reaches nothing.
// runAgent ends in log.Fatal, so the test runs the whole command in a child process.
func TestAgentWithoutAPIURL_StopsAndSaysWhy(t *testing.T) {
	if os.Getenv("SHELLY_PROVISIONER_TEST_CHILD") == "1" {
		os.Args = []string{"shelly-provisioner", "--config", os.Getenv("SHELLY_PROVISIONER_TEST_CONFIG"), "agent"}
		main()
		return
	}

	configPath := filepath.Join(t.TempDir(), "provisioner.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("{}\n"), 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAgentWithoutAPIURL_StopsAndSaysWhy$")
	cmd.Env = []string{
		"SHELLY_PROVISIONER_TEST_CHILD=1",
		"SHELLY_PROVISIONER_TEST_CONFIG=" + configPath,
		"HOME=" + t.TempDir(),
		"PATH=" + os.Getenv("PATH"),
	}
	out, err := cmd.CombinedOutput()

	require.NoError(t, ctx.Err(), "the agent kept running instead of stopping for the missing URL:\n%s", out)
	var exit *exec.ExitError
	require.True(t, errors.As(err, &exit), "want a non-zero exit, got %v:\n%s", err, out)
	require.NotZero(t, exit.ExitCode())
	require.True(t, strings.Contains(string(out), "API URL required"), "the message names the missing URL:\n%s", out)
}
