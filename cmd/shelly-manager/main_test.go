package main

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/testutil"
)

func TestCLICommands(t *testing.T) {
	// Build the binary for testing
	binaryPath := buildTestBinary(t)
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			t.Logf("Failed to remove binary: %v", err)
		}
	}()

	// Create temporary directories for test
	tempDir := testutil.TempDir(t)
	dbPath := filepath.Join(tempDir, "test.db")
	configPath := filepath.Join(tempDir, "testing.yaml")

	// Create test config file
	configContent := `server:
  port: 8081
  host: 127.0.0.1
  log_level: "error"

logging:
  level: "error"
  format: "text"
  output: "stderr"

database:
  path: ` + strconv.Quote(dbPath) + `

discovery:
  enabled: true
  enable_mdns: false
  networks:
    - 127.0.0.0/30
  timeout: 1
  concurrent_scans: 2
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Run("help command", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "--help")
		if err != nil {
			t.Fatalf("Help command failed: %v", err)
		}

		if !strings.Contains(output, "comprehensive tool") {
			t.Errorf("Expected help text to contain 'comprehensive tool', got: %s", output)
		}

		expectedCommands := []string{"add", "discover", "list", "provision", "scan-ap", "server"}
		for _, cmd := range expectedCommands {
			if !strings.Contains(output, cmd) {
				t.Errorf("Expected help to contain command '%s'", cmd)
			}
		}
	})

	t.Run("list empty database", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "--config", configPath, "list")
		if err != nil {
			t.Fatalf("List command failed: %v", err)
		}

		if !strings.Contains(output, "ID") || !strings.Contains(output, "IP") {
			t.Errorf("Expected list headers, got: %s", output)
		}
	})

	t.Run("discover command with timeout", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping network-dependent test in short mode")
		}

		// This scans the loopback only, and the config switches mDNS off: no packet leaves the host.
		// The scan sends GET /shelly to any listener on port 80 of 127.0.0.0/30, on this host.
		output, err := runCommandWithTimeout(t, 10*time.Second, binaryPath, "--config", configPath, "discover", "127.0.0.0/30")
		if err != nil {
			t.Fatalf("Discover command failed: %v", err)
		}

		if !strings.Contains(output, "Discovery complete") && !strings.Contains(output, "Found 0 devices") && !strings.Contains(output, "Discovering devices") {
			t.Errorf("Expected discovery completion message, got: %s", output)
		}
	})

	t.Run("provision command", func(t *testing.T) {
		// Test provision command without arguments (should show usage)
		output, err := runCommand(t, binaryPath, "--config", configPath, "provision")
		if err == nil {
			t.Error("Expected error for provision command without arguments")
		}

		// Should show usage information
		if !strings.Contains(output, "accepts between 1 and 2 arg(s)") && !strings.Contains(output, "Usage:") {
			t.Errorf("Expected usage error message, got: %s", output)
		}
	})

	// scan-ap lists WiFi networks by running the host's WiFi tool (nmcli on Linux), found through
	// PATH. A PATH with no such tool makes the scan fail with "executable file not found" and
	// report no devices, so the test never touches a real interface.
	t.Run("scan-ap command", func(t *testing.T) {
		emptyPath := t.TempDir()
		output, err := runCommandWithEnv(t, 30*time.Second, []string{"PATH=" + emptyPath}, binaryPath, "--config", configPath, "scan-ap")
		if err != nil {
			t.Fatalf("Scan-AP command failed: %v", err)
		}

		// Should complete and show scanning message
		if !strings.Contains(output, "Scanning for Shelly devices") && !strings.Contains(output, "No unprovisioned") {
			t.Errorf("Expected scanning message, got: %s", output)
		}
	})

	// Control for the case above: with a stand-in nmcli on PATH, scan-ap calls it. That shows
	// the PATH is what keeps the case above off the interface, and that the case above would
	// notice a call if the manager made one.
	t.Run("scan-ap command runs the WiFi tool found on PATH", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("scan-ap runs nmcli on Linux only")
		}

		shimDir := t.TempDir()
		callLog := filepath.Join(shimDir, "calls.log")
		// The log path goes in through the environment: a path put into the script text would be
		// read as shell code.
		shim := "#!/bin/sh\necho \"$@\" >> \"$NMCLI_CALL_LOG\"\nexit 1\n"
		if err := os.WriteFile(filepath.Join(shimDir, "nmcli"), []byte(shim), 0755); err != nil {
			t.Fatalf("Failed to write nmcli stand-in: %v", err)
		}

		env := []string{"PATH=" + shimDir, "NMCLI_CALL_LOG=" + callLog}
		if _, err := runCommandWithEnv(t, 30*time.Second, env, binaryPath, "--config", configPath, "scan-ap"); err != nil {
			t.Fatalf("Scan-AP command failed: %v", err)
		}

		calls, err := os.ReadFile(callLog)
		if err != nil {
			t.Fatalf("Expected scan-ap to call nmcli on PATH, but the stand-in logged nothing: %v", err)
		}
		if len(calls) == 0 {
			t.Error("Expected scan-ap to call nmcli on PATH, but the stand-in log is empty")
		}
	})

	t.Run("invalid command", func(t *testing.T) {
		_, err := runCommand(t, binaryPath, "invalidcommand")
		if err == nil {
			t.Error("Expected error for invalid command")
		}
	})
}

func TestConfigFileHandling(t *testing.T) {
	binaryPath := buildTestBinary(t)
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			t.Logf("Failed to remove binary: %v", err)
		}
	}()

	t.Run("missing config file", func(t *testing.T) {
		out, err := runCommand(t, binaryPath, "--config", "/nonexistent/config.yaml", "list")
		if err == nil {
			t.Error("Expected error for missing config file")
		}
		// A file that is missing is named, with the OS's own cause, as it always was.
		if !strings.Contains(out, "Failed to load config: failed to read config file at '/nonexistent/config.yaml'") ||
			!strings.Contains(out, "no such file or directory") || strings.Contains(out, "details withheld") {
			t.Errorf("a missing file must be named with its cause, got:\n%s", out)
		}
	})

	// The YAML decoder quotes the value it could not decode, so an admin key written in a shape that does not load
	// (a tag that makes it an integer) would reach the log whole (ginsys/shelly-manager#872). The shapes are in
	// internal/config; here the binary shows it prints what the loader returns: the path and one fixed sentence.
	// One shape fails in each decoder: a tag that makes the value an integer (the YAML decoder), a map (the settings
	// decoder), and a SHELLY_ variable of the wrong type (the settings decoder again). Each prints one line, the path
	// and the fixed sentence, with nothing else and nothing of the secret.
	t.Run("a config that does not load never prints a secret that is in it", func(t *testing.T) {
		const secret = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
		const sentence = "the configuration is not valid (details withheld, they may quote a secret): check the file's syntax with a YAML linter, compare each setting's type with the sample config (a linter finds a syntax error, never a wrong type), and check that each SHELLY_ environment variable holds a value of the setting's type"
		for name, tc := range map[string]struct {
			body string
			env  []string
		}{
			"tagged":    {"security:\n  admin_api_key: !!int " + secret + "\n", nil},
			"a map":     {"security:\n  admin_api_key: {a: " + secret + "}\n", nil},
			"bad value": {"server:\n  host: 127.0.0.1\n", []string{"SHELLY_SERVER_PORT=port" + secret}},
		} {
			path := filepath.Join(testutil.TempDir(t), "secret.yaml")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}

			out, err := runCommandWithEnv(t, 30*time.Second, tc.env, binaryPath, "--config", path, "list")
			if err == nil {
				t.Errorf("%s: expected an error for a config that does not load", name)
			}
			for _, piece := range []string{secret[:16], secret[16:32], secret[32:48], secret[48:]} {
				if strings.Contains(out, piece) {
					t.Errorf("%s: the output holds a piece of the secret %q:\n%s", name, piece, out)
				}
			}
			if !strings.Contains(out, "Failed to load config: "+path+": "+sentence+"\n") {
				t.Errorf("%s: want the path and one fixed sentence, got:\n%s", name, out)
			}
			if strings.Count(out, "\n") != 1 {
				t.Errorf("%s: nothing else may be printed, got:\n%s", name, out)
			}
		}
	})

	t.Run("invalid config file", func(t *testing.T) {
		tempDir := testutil.TempDir(t)
		invalidConfigPath := filepath.Join(tempDir, "invalid.yaml")

		err := os.WriteFile(invalidConfigPath, []byte("invalid: yaml: content: ["), 0644)
		if err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		_, err = runCommand(t, binaryPath, "--config", invalidConfigPath, "list")
		if err == nil {
			t.Error("Expected error for invalid config file")
		}
	})
}

func TestServerCommand(t *testing.T) {
	binaryPath := buildTestBinary(t)
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			t.Logf("Failed to remove binary: %v", err)
		}
	}()

	tempDir := testutil.TempDir(t)
	configPath := filepath.Join(tempDir, "server-config.yaml")

	// Use a unique port to avoid conflicts
	configContent := `server:
  port: 8082
  host: 127.0.0.1
  log_level: "error"

logging:
  level: "error"
  format: "text"
  output: "stderr"

database:
  path: ` + strconv.Quote(filepath.Join(tempDir, "server.db")) + `

discovery:
  enabled: false
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Start server in background
	cmd := exec.Command(binaryPath, "--config", configPath, "server")

	// Capture output
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for the server to start by polling the health endpoint. A busy machine can take
	// several seconds, so allow 15 seconds before giving up. The route is /healthz: the
	// server has no /health, which this test used to poll and always got a 404 from.
	serverURL := "http://127.0.0.1:8082/healthz"
	serverReady := false

	// One deadline bounds the sleeps and the requests, and a request that stalls cannot
	// outlast its own timeout, so the test always reaches the kill below.
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(15 * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		resp, err := client.Get(serverURL)
		if err == nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Logf("Failed to close response body: %v", closeErr)
			}
			if resp.StatusCode == http.StatusOK {
				serverReady = true
				break
			}
		}
	}

	// Kill the server
	if killErr := cmd.Process.Kill(); killErr != nil {
		t.Logf("Failed to kill process: %v", killErr)
	}
	if waitErr := cmd.Wait(); waitErr != nil {
		t.Logf("Process wait error: %v", waitErr)
	}

	output := out.String()
	if !serverReady {
		t.Fatalf("Server did not answer %s within 15 seconds, output: %s", serverURL, output)
	}
	if !strings.Contains(output, "Starting server") {
		t.Errorf("Expected server startup message, got: %s", output)
	}
}

func TestAddCommand(t *testing.T) {
	binaryPath := buildTestBinary(t)
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			t.Logf("Failed to remove binary: %v", err)
		}
	}()

	tempDir := testutil.TempDir(t)
	dbPath := filepath.Join(tempDir, "add-test.db")
	configPath := filepath.Join(tempDir, "add-config.yaml")

	configContent := `server:
  port: 8083
  host: 127.0.0.1
  log_level: "error"

logging:
  level: "error"
  format: "text"
  output: "stderr"

database:
  path: ` + strconv.Quote(dbPath) + `

discovery:
  enabled: true
  timeout: 1
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Run("add command without arguments", func(t *testing.T) {
		_, err := runCommand(t, binaryPath, "--config", configPath, "add")
		if err == nil {
			t.Error("Expected error for add command without arguments")
		}
	})

	// The address is checked before any probe, so this reaches no network. A run against an
	// address that no device answers is covered in discover_added_test.go with a fake probe: the
	// binary has no fake, and a real probe would leave the machine.
	t.Run("add command with invalid IP", func(t *testing.T) {
		_, err := runCommand(t, binaryPath, "--config", configPath, "add", "invalid-ip")
		if err == nil {
			t.Error("Expected error for add command with invalid IP")
		}
	})
}

func TestCommandLineArguments(t *testing.T) {
	binaryPath := buildTestBinary(t)
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			t.Logf("Failed to remove binary: %v", err)
		}
	}()

	t.Run("version information", func(t *testing.T) {
		output, _ := runCommand(t, binaryPath, "--help")
		// Should contain basic app information
		if !strings.Contains(output, "shelly-manager") {
			t.Errorf("Expected app name in help output, got: %s", output)
		}
	})

	t.Run("config flag validation", func(t *testing.T) {
		_, err := runCommand(t, binaryPath, "--config", "/nonexistent/file.yaml", "list")
		if err == nil {
			t.Error("Expected error for nonexistent config file")
		}
	})
}

func TestCommandExecution(t *testing.T) {
	binaryPath := buildTestBinary(t)
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			t.Logf("Failed to remove binary: %v", err)
		}
	}()

	tempDir := testutil.TempDir(t)
	configPath := filepath.Join(tempDir, "exec-config.yaml")

	configContent := `server:
  port: 8084
  host: 127.0.0.1
  log_level: "error"

logging:
  level: "error"
  format: "text"
  output: "stderr"

database:
  path: ` + strconv.Quote(filepath.Join(tempDir, "exec.db")) + `

discovery:
  enabled: true
  enable_mdns: false
  timeout: 1
  networks:
    - 127.0.0.0/30
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Run("discover with specific network", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping network-dependent test in short mode")
		}

		output, err := runCommandWithTimeout(t, 10*time.Second, binaryPath, "--config", configPath, "discover", "127.0.0.0/30")
		if err != nil {
			t.Fatalf("Discover command with network failed: %v", err)
		}

		// Should show discovery process
		if !strings.Contains(output, "Discovering") && !strings.Contains(output, "Discovery complete") && !strings.Contains(output, "Found 0 devices") {
			t.Errorf("Expected discovery output, got: %s", output)
		}
	})

	t.Run("list with devices", func(t *testing.T) {
		// First run list to ensure table headers are shown
		output, err := runCommand(t, binaryPath, "--config", configPath, "list")
		if err != nil {
			t.Fatalf("List command failed: %v", err)
		}

		// Should show table structure even if empty
		if !strings.Contains(output, "ID") {
			t.Errorf("Expected table headers in list output, got: %s", output)
		}
	})
}

func TestErrorHandling(t *testing.T) {
	binaryPath := buildTestBinary(t)
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			t.Logf("Failed to remove binary: %v", err)
		}
	}()

	t.Run("unknown flag", func(t *testing.T) {
		_, err := runCommand(t, binaryPath, "--unknown-flag")
		if err == nil {
			t.Error("Expected error for unknown flag")
		}
	})

	t.Run("invalid command", func(t *testing.T) {
		_, err := runCommand(t, binaryPath, "nonexistent-command")
		if err == nil {
			t.Error("Expected error for nonexistent command")
		}
	})
}

// Helper functions

func buildTestBinary(t *testing.T) string {
	t.Helper()

	tempDir := testutil.TempDir(t)
	binaryName := "shelly-manager-test"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tempDir, binaryName)

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build test binary: %v\nOutput: %s", err, output)
	}

	return binaryPath
}

func runCommand(t *testing.T, binary string, args ...string) (string, error) {
	t.Helper()
	return runCommandWithTimeout(t, 30*time.Second, binary, args...)
}

func runCommandWithTimeout(t *testing.T, timeout time.Duration, binary string, args ...string) (string, error) {
	t.Helper()
	return runCommandWithEnv(t, timeout, nil, binary, args...)
}

// runCommandWithEnv runs the binary with extraEnv added after the caller's environment (a later
// entry for the same variable wins).
func runCommandWithEnv(t *testing.T, timeout time.Duration, extraEnv []string, binary string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(binary, args...)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Set CGO_ENABLED for SQLite. The manager reads SHELLY_* variables over its config file, so
	// pin mDNS off here: a variable in the caller's environment must not send multicast from a test.
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "SHELLY_DISCOVERY_ENABLE_MDNS=false")
	cmd.Env = append(cmd.Env, extraEnv...)

	done := make(chan error, 1)

	err := cmd.Start()
	if err != nil {
		return "", err
	}

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return out.String(), err
	case <-time.After(timeout):
		if cmd.Process != nil {
			if killErr := cmd.Process.Kill(); killErr != nil {
				t.Logf("Failed to kill process: %v", killErr)
			}
		}
		if waitErr := <-done; waitErr != nil {
			t.Logf("process wait after kill: %v", waitErr)
		}
		return out.String(), &TimeoutError{timeout}
	}
}

type TimeoutError struct {
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return "command timed out after " + e.Timeout.String()
}

// Benchmark tests
func BenchmarkListCommand(b *testing.B) {
	binaryPath := buildTestBinary(&testing.T{})
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			b.Logf("Failed to remove binary: %v", err)
		}
	}()

	tempDir := testutil.TempDir(&testing.T{})
	configPath := filepath.Join(tempDir, "bench-config.yaml")

	configContent := `server:
  port: 8083
  host: 127.0.0.1
  log_level: "error"

logging:
  level: "error"
  format: "text"
  output: "stderr"

database:
  path: ` + strconv.Quote(filepath.Join(tempDir, "bench.db")) + `
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		b.Fatalf("Failed to write config file: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cmd := exec.Command(binaryPath, "--config", configPath, "list")
		err := cmd.Run()
		if err != nil {
			b.Fatalf("Command failed: %v", err)
		}
	}
}
