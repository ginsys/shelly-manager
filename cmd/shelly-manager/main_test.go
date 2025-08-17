package main

import (
	"bytes"
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
	defer os.Remove(binaryPath)

	// Create temporary directories for test
	tempDir := testutil.TempDir(t)
	dbPath := filepath.Join(tempDir, "test.db")
	configPath := filepath.Join(tempDir, "test-config.yaml")

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
  networks:
    - 192.168.1.0/30
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

		// This should complete quickly since we're using a small network range
		output, err := runCommandWithTimeout(t, 10*time.Second, binaryPath, "--config", configPath, "discover", "192.168.1.0/30")
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

	t.Run("scan-ap command", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping network-dependent test in short mode")
		}

		output, err := runCommand(t, binaryPath, "--config", configPath, "scan-ap")
		if err != nil {
			t.Fatalf("Scan-AP command failed: %v", err)
		}

		// Should complete and show scanning message
		if !strings.Contains(output, "Scanning for Shelly devices") && !strings.Contains(output, "No unprovisioned") {
			t.Errorf("Expected scanning message, got: %s", output)
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
	defer os.Remove(binaryPath)

	t.Run("missing config file", func(t *testing.T) {
		_, err := runCommand(t, binaryPath, "--config", "/nonexistent/config.yaml", "list")
		if err == nil {
			t.Error("Expected error for missing config file")
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
	defer os.Remove(binaryPath)

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

	// Give server time to start
	time.Sleep(2 * time.Second)

	// Test that server is running by trying to connect
	// (We're not doing a full HTTP test here, just checking it starts)

	// Kill the server
	if cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
	}

	output := out.String()
	if !strings.Contains(output, "Starting server") {
		t.Errorf("Expected server startup message, got: %s", output)
	}
}

func TestAddCommand(t *testing.T) {
	binaryPath := buildTestBinary(t)
	defer os.Remove(binaryPath)

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

	t.Run("add command with invalid IP", func(t *testing.T) {
		_, err := runCommand(t, binaryPath, "--config", configPath, "add", "invalid-ip")
		if err == nil {
			t.Error("Expected error for add command with invalid IP")
		}
	})

	t.Run("add command with unreachable IP", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping network-dependent test in short mode")
		}

		// Use a non-routable IP address that will timeout quickly
		_, err := runCommandWithTimeout(t, 5*time.Second, binaryPath, "--config", configPath, "add", "10.255.255.254")
		if err == nil {
			t.Error("Expected error for add command with unreachable IP")
		}
	})
}

func TestCommandLineArguments(t *testing.T) {
	binaryPath := buildTestBinary(t)
	defer os.Remove(binaryPath)

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
	defer os.Remove(binaryPath)

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
  timeout: 1
  networks:
    - 192.168.1.0/30
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	t.Run("discover with specific network", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping network-dependent test in short mode")
		}

		output, err := runCommandWithTimeout(t, 10*time.Second, binaryPath, "--config", configPath, "discover", "192.168.1.0/30")
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
	defer os.Remove(binaryPath)

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

	cmd := exec.Command(binary, args...)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	// Set CGO_ENABLED for SQLite
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")

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
			cmd.Process.Kill()
		}
		<-done // Wait for the process to be killed
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
	defer os.Remove(binaryPath)

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
