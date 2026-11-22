//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/agentkey"
	"github.com/ginsys/shelly-manager/internal/provisioning"
)

// The agent's seed and key (ginsys/shelly-manager#638, slice 3). Every case runs the real binary, because the
// promises are about what a process prints, logs and exits with: the seed never leaves it, `seed` and
// `fingerprint` print their value and nothing else, and a bad setting stops the start with a message that
// never repeats the value. The seed below is the one whose key, fingerprint and signature
// internal/agentkey pins as literals.

const (
	cmdSeed     = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	cmdReadable = "5647-5aa7-5463-474c-0285-df5d-bf2b-cab7"
	cmdSeedVar  = "SHELLY_PROVISIONING_AGENT_SEED"
	// The agent id for cmdSeed, one fixed value (never computed from the code under test).
	cmdAgentID = "keyed-56475aa75463474c0285df5dbf2bcab7"

	// notValidSentence is all a config that does not load may show beside its path: nothing in it comes from the file.
	notValidSentence = "the configuration is not valid (details withheld, they may quote a secret): check the file's syntax with a YAML linter, compare each setting's type with the sample config (a linter finds a syntax error, never a wrong type), and check that each SHELLY_ environment variable holds a value of the setting's type"
)

// syncBuffer is the child's whole output (stdout and stderr together), safe to read while it runs.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// provisionerRun builds the binary once and gives each case an environment of its own: an empty HOME, a PATH
// that holds only a stand-in nmcli (no real Wi-Fi device is read or changed), and the variables the case sets.
type provisionerRun struct {
	binary, home, shimDir string
}

func newProvisionerRun(t *testing.T) *provisionerRun {
	t.Helper()
	goBin, err := exec.LookPath("go")
	require.NoError(t, err)
	binary := filepath.Join(t.TempDir(), "shelly-provisioner")
	// -trimpath keeps the binary from finding the sample config in this source tree (config.LoadWithName adds a
	// path made from the file name the compiler recorded), as an installed binary cannot.
	out, err := exec.Command(goBin, "build", "-trimpath", "-o", binary, ".").CombinedOutput()
	require.NoError(t, err, string(out))
	shimDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(shimDir, "nmcli"), []byte("#!/bin/sh\nexit 1\n"), 0o755))
	return &provisionerRun{binary: binary, home: t.TempDir(), shimDir: shimDir}
}

func (p *provisionerRun) command(ctx context.Context, env []string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.binary, args...)
	cmd.Dir = p.home
	cmd.Env = append([]string{"HOME=" + p.home, "PATH=" + p.shimDir}, env...)
	return cmd
}

// run runs a command that ends by itself and returns its output and exit code.
func (p *provisionerRun) run(t *testing.T, env []string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := p.command(ctx, env, args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "the command kept running:\n%s", out)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode()
	}
	require.NoError(t, err, string(out))
	return string(out), 0
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "provisioner.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// fakeRegisterManager records every register body and answers each with the given status ("pending" or
// "rejected"), in the shape the real handler writes.
type fakeRegisterManager struct {
	mu     sync.Mutex
	bodies []map[string]any
	srv    *httptest.Server
}

func newFakeRegisterManager(t *testing.T, status string) *fakeRegisterManager {
	t.Helper()
	f := &fakeRegisterManager{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.bodies = append(f.bodies, body)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
			"status": status, "agent_id": cmdAgentID, "fingerprint": cmdReadable, "message": "the operator decides",
		}})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRegisterManager) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.bodies)
}

func (f *fakeRegisterManager) waitFor(t *testing.T, n int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for f.count() < n {
		require.True(t, time.Now().Before(deadline), "the agent made %d register requests, want %d", f.count(), n)
		time.Sleep(50 * time.Millisecond)
	}
}

// startAgent starts `agent` and returns a stop function that ends it with SIGTERM and returns its output.
func (p *provisionerRun) startAgent(t *testing.T, env []string, args ...string) (output func() string, stop func() string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	out := &syncBuffer{}
	cmd := p.command(ctx, env, append(args, "agent")...)
	cmd.Stdout, cmd.Stderr = out, out
	require.NoError(t, cmd.Start())
	stopped := false
	stop = func() string {
		if !stopped {
			stopped = true
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func() { _ = cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				cancel()
				<-done
			}
		}
		return out.String()
	}
	t.Cleanup(func() { _ = stop() })
	return out.String, stop
}

func TestKeyedAgentCommands(t *testing.T) {
	p := newProvisionerRun(t)
	badConfig := writeConfig(t, "this: [is: not yaml\n")

	t.Run("seed prints one random 64-hex line and reads no config", func(t *testing.T) {
		first, code := p.run(t, nil, "--config", badConfig, "seed")
		require.Zero(t, code, first)
		second, code := p.run(t, nil, "--config", badConfig, "seed")
		require.Zero(t, code, second)
		assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}\n$`), first, "one line, the seed and nothing else")
		assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}\n$`), second)
		assert.NotEqual(t, first, second)
		_, err := agentkey.FromSeed(strings.TrimSpace(first))
		assert.NoError(t, err, "what `seed` prints is a seed the agent takes")
	})

	t.Run("fingerprint prints the fingerprint from the seed alone", func(t *testing.T) {
		out, code := p.run(t, []string{cmdSeedVar + "=" + cmdSeed}, "fingerprint")
		require.Zero(t, code, out)
		assert.Equal(t, cmdReadable+"\n", out, "eight groups of four, one line, nothing else")
	})

	t.Run("fingerprint needs no other secret", func(t *testing.T) {
		out, code := p.run(t, []string{cmdSeedVar + "=" + cmdSeed, "SHELLY_API_KEY_FILE=/no/such/file"}, "fingerprint")
		require.Zero(t, code, out)
		assert.Equal(t, cmdReadable+"\n", out, "an unreadable key file for another secret does not stop a command that works from the seed alone")
	})

	t.Run("the seed is read from its _FILE form as well", func(t *testing.T) {
		seedFile := filepath.Join(t.TempDir(), "seed")
		require.NoError(t, os.WriteFile(seedFile, []byte(cmdSeed+"\n"), 0o600))
		out, code := p.run(t, []string{cmdSeedVar + "_FILE=" + seedFile}, "fingerprint")
		require.Zero(t, code, out)
		assert.Equal(t, cmdReadable+"\n", out)
	})

	t.Run("fingerprint with no seed stops and names the variable and the setting", func(t *testing.T) {
		out, code := p.run(t, nil, "fingerprint")
		assert.NotZero(t, code)
		assert.Contains(t, out, cmdSeedVar)
		assert.Contains(t, out, "provisioning.agent_seed")
	})

	// The command exists so an operator can pre-approve the fingerprint the agent will show, so it finds the seed
	// where the agent does: the config file, then the variable or its _FILE form (the variable wins).
	t.Run("fingerprint reads the seed the way the agent does", func(t *testing.T) {
		config := writeConfig(t, "provisioning:\n  agent_seed: "+cmdSeed+"\n")
		out, code := p.run(t, nil, "--config", config, "fingerprint")
		require.Zero(t, code, out)
		assert.Equal(t, cmdReadable+"\n", out, "a seed that is only in the config file")

		other := strings.Repeat("ab", 32)
		out, code = p.run(t, []string{cmdSeedVar + "=" + other}, "--config", config, "fingerprint")
		require.Zero(t, code, out)
		assert.NotEqual(t, cmdReadable+"\n", out, "the variable wins over the file, as for the agent")

		bad := writeConfig(t, "provisioning:\n  agent_seed: "+strings.Repeat("zz", 32)+"\n")
		out, code = p.run(t, nil, "--config", bad, "fingerprint")
		assert.NotZero(t, code)
		assert.NotContains(t, out, strings.Repeat("zz", 32), "a malformed seed from the config file is never echoed either")
	})

	t.Run("a malformed seed stops every command that reads it, and no message repeats it", func(t *testing.T) {
		for name, bad := range map[string]string{
			"not hex":   strings.Repeat("zz", 32),
			"too short": cmdSeed[:62],
			"too long":  cmdSeed + "00",
		} {
			for _, args := range [][]string{{"fingerprint"}, {"--config", writeConfig(t, "{}\n"), "--api-url", "http://127.0.0.1:1", "agent"}} {
				out, code := p.run(t, []string{cmdSeedVar + "=" + bad}, args...)
				assert.NotZero(t, code, "%s, %v:\n%s", name, args, out)
				assert.Contains(t, out, cmdSeedVar, "%s, %v: the message names the setting", name, args)
				assert.NotContains(t, out, bad, "%s, %v: the message never carries the value", name, args)
				assert.NotContains(t, out, bad[:16], "%s, %v", name, args)
			}
		}
	})

	t.Run("a config that does not load never echoes a seed that is in the environment", func(t *testing.T) {
		out, code := p.run(t, []string{cmdSeedVar + "=" + cmdSeed}, "--config", badConfig, "--api-url", "http://127.0.0.1:1", "agent")
		assert.NotZero(t, code, out)
		assert.NotContains(t, out, cmdSeed)
		assert.NotContains(t, out, cmdSeed[:16])
	})

	// The loader withholds what the decoders quote, in every shape of the value: that matrix runs in internal/config
	// (load_failure_test.go). Here the binary shows both commands print what the loader returns: the file's path and
	// the fixed sentence, and nothing of the seed (a tag that makes the value an integer, which the YAML decoder quotes).
	t.Run("a config that fails to parse never prints a seed that is in it", func(t *testing.T) {
		groups := []string{cmdSeed[:16], cmdSeed[16:32], cmdSeed[32:48], cmdSeed[48:]}
		config := writeConfig(t, "logging:\n  level: info\nprovisioning:\n  agent_seed: !!int "+cmdSeed+"\n")
		for _, args := range [][]string{{"fingerprint"}, {"--api-url", "http://127.0.0.1:1", "agent"}} {
			out, code := p.run(t, nil, append([]string{"--config", config}, args...)...)
			assert.NotZero(t, code, "%v:\n%s", args, out)
			for _, piece := range groups {
				assert.NotContains(t, out, piece, "%v", args)
			}
			assert.Contains(t, out, "Failed to load config: "+config+": "+notValidSentence+"\n", "%v: the path and one fixed sentence", args)
		}
	})

	// A file that is missing is still named, with the OS's own cause, on both commands.
	t.Run("a config file that is missing is named", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "absent.yaml")
		for _, args := range [][]string{{"fingerprint"}, {"--api-url", "http://127.0.0.1:1", "agent"}} {
			out, code := p.run(t, nil, append([]string{"--config", missing}, args...)...)
			assert.NotZero(t, code, "%v:\n%s", args, out)
			assert.Contains(t, out, "Failed to load config: failed to read config file at '"+missing+"'", "%v", args)
			assert.Contains(t, out, "no such file or directory", "%v", args)
			assert.NotContains(t, out, "details withheld", "%v", args)
		}
	})

	t.Run("strict with no seed refuses to start, and with a seed starts", func(t *testing.T) {
		config := writeConfig(t, "provisioning:\n  enrolment: strict\n")
		out, code := p.run(t, nil, "--config", config, "--api-url", "http://127.0.0.1:1", "agent")
		assert.NotZero(t, code, out)
		assert.Contains(t, out, "provisioning.enrolment")
		assert.Contains(t, out, "agent_seed")

		mgr := newFakeRegisterManager(t, "pending")
		_, stop := p.startAgent(t, []string{cmdSeedVar + "=" + cmdSeed}, "--config", config, "--api-url", mgr.srv.URL)
		mgr.waitFor(t, 1, 15*time.Second)
		assert.NotContains(t, stop(), "provisioning.enrolment")
	})
}

// The seed never leaves the process. The agent runs at debug level against a manager that answers `pending`,
// then `status` runs with the same environment; the seed, and so the private key it makes, is in none of the
// output of either (stdout, stderr and the log together) and in no request body.
func TestKeyedAgent_TheSeedNeverLeaves(t *testing.T) {
	p := newProvisionerRun(t)
	mgr := newFakeRegisterManager(t, "pending")
	config := writeConfig(t, "logging:\n  level: debug\n  format: text\n  output: stdout\n")
	env := []string{cmdSeedVar + "=" + cmdSeed, "SHELLY_API_KEY=agent-secret"}

	_, stop := p.startAgent(t, env, "--config", config, "--api-url", mgr.srv.URL)
	mgr.waitFor(t, 1, 15*time.Second)
	agentOut := stop()
	statusOut, code := p.run(t, env, "--config", config, "--api-url", mgr.srv.URL, "status")
	require.Zero(t, code, statusOut)

	for name, out := range map[string]string{"agent": agentOut, "status": statusOut} {
		assert.NotContains(t, out, cmdSeed, name)
		assert.NotContains(t, out, cmdSeed[:16], name)
		assert.Contains(t, out, cmdReadable, "%s prints the fingerprint the operator will look for", name)
		assert.Contains(t, out, cmdAgentID, "%s names the agent by the id the key derives", name)
		assert.NotRegexp(t, `provisioner-.*-\d+`, out, "%s: a keyed agent has no pid-based id", name)
	}
	raw, err := json.Marshal(mgr.bodies)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), cmdSeed, "the seed is never sent")
	assert.Contains(t, string(raw), "03a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8", "the public key is")
}

// With no seed the agent is the agent it always was: a pid-based id and no key in the request.
func TestKeylessAgent_RegistersAsBefore(t *testing.T) {
	p := newProvisionerRun(t)
	mgr := newFakeRegisterManager(t, "pending")
	out, stop := p.startAgent(t, []string{"SHELLY_API_KEY=agent-secret"}, "--config", writeConfig(t, "{}\n"), "--api-url", mgr.srv.URL)
	mgr.waitFor(t, 1, 15*time.Second)
	_ = stop()

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	assert.Regexp(t, `^provisioner-.*-\d+$`, mgr.bodies[0]["id"])
	assert.NotContains(t, mgr.bodies[0], "public_key")
	assert.Regexp(t, `Agent ID: provisioner-.*-\d+`, out())
}

// `pending` waits 30 seconds whatever provision_interval says; `rejected` logs an error naming the
// fingerprint and retries at the poll interval (the design note; RULING-20261122-0421-638h).
func TestKeyedAgent_RetryPacing(t *testing.T) {
	p := newProvisionerRun(t)
	config := writeConfig(t, "provisioning:\n  provision_interval: 1\n")
	env := []string{cmdSeedVar + "=" + cmdSeed, "SHELLY_API_KEY=agent-secret"}

	t.Run("pending does not retry at the poll interval", func(t *testing.T) {
		mgr := newFakeRegisterManager(t, "pending")
		_, stop := p.startAgent(t, env, "--config", config, "--api-url", mgr.srv.URL)
		mgr.waitFor(t, 1, 15*time.Second)
		time.Sleep(3 * time.Second)
		assert.Equal(t, 1, mgr.count(), "a poll interval of 1 s must not turn into a request a second for an operator who has not looked yet")
		_ = stop()
	})

	t.Run("rejected logs an error with the fingerprint and retries at the poll interval", func(t *testing.T) {
		mgr := newFakeRegisterManager(t, "rejected")
		_, stop := p.startAgent(t, env, "--config", config, "--api-url", mgr.srv.URL)
		mgr.waitFor(t, 2, 15*time.Second)
		out := stop()
		var errorLine string
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "level=ERROR") && strings.Contains(line, cmdReadable) {
				errorLine = line
			}
		}
		assert.NotEmpty(t, errorLine, "an ERROR line names the fingerprint:\n%s", out)
	})
}

func TestNextPollDelay(t *testing.T) {
	const poll = 7 * time.Second
	assert.Equal(t, 30*time.Second, nextPollDelay(&provisioning.NotEnrolledError{Status: "pending"}, poll), "pending: 30 s, whatever the interval")
	assert.Equal(t, 30*time.Second, nextPollDelay(&provisioning.NotEnrolledError{Status: "pending"}, 120*time.Second), "pending never waits longer than 30 s either")
	assert.Equal(t, poll, nextPollDelay(&provisioning.NotEnrolledError{Status: "rejected"}, poll))
	assert.Equal(t, poll, nextPollDelay(errors.New("connection refused"), poll))
	assert.Equal(t, poll, nextPollDelay(nil, poll))
}
