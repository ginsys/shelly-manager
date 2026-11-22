package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Security monitoring and IP blocking are keys, off by default (ginsys/shelly-manager#861).

func loadSecurityYAML(t *testing.T, body string) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

// Check 2: no key set, nothing on, and the numbers are the ones the monitor always used.
func TestSecurityMonitoringIsOffByDefault(t *testing.T) {
	cfg := loadSecurityYAML(t, "server:\n  port: 8080\n")
	sec := cfg.Security
	if sec.Monitoring.Enabled || sec.IPBlocking.Enabled {
		t.Fatalf("monitoring=%v ip_blocking=%v, want both off", sec.Monitoring.Enabled, sec.IPBlocking.Enabled)
	}
	ib := sec.IPBlocking
	if ib.BlockSeconds != 3600 || ib.SuspiciousThreshold != 10 || ib.RateLimitThreshold != 5 ||
		ib.AttackTypeThreshold != 3 || ib.SuspiciousPerHourThreshold != 5 {
		t.Fatalf("defaults = %+v", ib)
	}
	if err := cfg.ValidateSecurityMonitoring(); err != nil {
		t.Fatalf("the defaults must be valid: %v", err)
	}
}

// Check 3: each of the seven keys is read from the YAML file, and from its environment variable.
func TestSecurityMonitoringKeysFromYAMLAndEnv(t *testing.T) {
	yamlBody := `
security:
  monitoring:
    enabled: true
  ip_blocking:
    enabled: true
    block_seconds: 172800
    suspicious_threshold: 21
    rate_limit_threshold: 22
    attack_type_threshold: 23
    suspicious_per_hour_threshold: 24
`
	check := func(t *testing.T, cfg *Config) {
		t.Helper()
		ib := cfg.Security.IPBlocking
		if !cfg.Security.Monitoring.Enabled || !ib.Enabled {
			t.Errorf("switches: monitoring=%v ip_blocking=%v", cfg.Security.Monitoring.Enabled, ib.Enabled)
		}
		if ib.BlockSeconds != 172800 || ib.SuspiciousThreshold != 21 || ib.RateLimitThreshold != 22 ||
			ib.AttackTypeThreshold != 23 || ib.SuspiciousPerHourThreshold != 24 {
			t.Errorf("numbers = %+v", ib)
		}
	}

	t.Run("yaml", func(t *testing.T) {
		check(t, loadSecurityYAML(t, yamlBody))
	})
	t.Run("env", func(t *testing.T) {
		t.Setenv("SHELLY_SECURITY_MONITORING_ENABLED", "true")
		t.Setenv("SHELLY_SECURITY_IP_BLOCKING_ENABLED", "true")
		t.Setenv("SHELLY_SECURITY_IP_BLOCKING_BLOCK_SECONDS", "172800")
		t.Setenv("SHELLY_SECURITY_IP_BLOCKING_SUSPICIOUS_THRESHOLD", "21")
		t.Setenv("SHELLY_SECURITY_IP_BLOCKING_RATE_LIMIT_THRESHOLD", "22")
		t.Setenv("SHELLY_SECURITY_IP_BLOCKING_ATTACK_TYPE_THRESHOLD", "23")
		t.Setenv("SHELLY_SECURITY_IP_BLOCKING_SUSPICIOUS_PER_HOUR_THRESHOLD", "24")
		check(t, loadSecurityYAML(t, "server:\n  port: 8080\n"))
	})
}

// Checks 7 and 8: a start-up stops, and the error names the key.
func TestSecurityMonitoringValidation(t *testing.T) {
	on := func(c *Config) { c.Security.Monitoring.Enabled, c.Security.IPBlocking.Enabled = true, true }
	valid := func() *Config {
		c := &Config{}
		on(c)
		c.Security.IPBlocking.BlockSeconds = 3600
		c.Security.IPBlocking.SuspiciousThreshold = 10
		c.Security.IPBlocking.RateLimitThreshold = 5
		c.Security.IPBlocking.AttackTypeThreshold = 3
		c.Security.IPBlocking.SuspiciousPerHourThreshold = 5
		return c
	}

	if err := valid().ValidateSecurityMonitoring(); err != nil {
		t.Fatalf("valid configuration refused: %v", err)
	}

	blockingWithoutMonitor := valid()
	blockingWithoutMonitor.Security.Monitoring.Enabled = false
	err := blockingWithoutMonitor.ValidateSecurityMonitoring()
	if err == nil || !strings.Contains(err.Error(), "security.ip_blocking.enabled") || !strings.Contains(err.Error(), "security.monitoring.enabled") {
		t.Errorf("blocking without the monitor: err = %v, want both keys named", err)
	}

	monitorOnlyIsFine := valid()
	monitorOnlyIsFine.Security.IPBlocking.Enabled = false
	if err = monitorOnlyIsFine.ValidateSecurityMonitoring(); err != nil {
		t.Errorf("monitoring on, blocking off: %v", err)
	}

	// The hourly threshold has an upper bound too: both sides of it.
	atBound := valid()
	atBound.Security.IPBlocking.SuspiciousPerHourThreshold = MaxSuspiciousPerHourThreshold
	if err = atBound.ValidateSecurityMonitoring(); err != nil {
		t.Errorf("hourly threshold at the bound (%d): %v", MaxSuspiciousPerHourThreshold, err)
	}
	overBound := valid()
	overBound.Security.IPBlocking.SuspiciousPerHourThreshold = MaxSuspiciousPerHourThreshold + 1
	err = overBound.ValidateSecurityMonitoring()
	if err == nil || !strings.Contains(err.Error(), "security.ip_blocking.suspicious_per_hour_threshold") || !strings.Contains(err.Error(), "1000") {
		t.Errorf("hourly threshold over the bound: err = %v, want the key and the limit named", err)
	}

	fields := map[string]func(*Config, int){
		"security.ip_blocking.block_seconds":                 func(c *Config, v int) { c.Security.IPBlocking.BlockSeconds = v },
		"security.ip_blocking.suspicious_threshold":          func(c *Config, v int) { c.Security.IPBlocking.SuspiciousThreshold = v },
		"security.ip_blocking.rate_limit_threshold":          func(c *Config, v int) { c.Security.IPBlocking.RateLimitThreshold = v },
		"security.ip_blocking.attack_type_threshold":         func(c *Config, v int) { c.Security.IPBlocking.AttackTypeThreshold = v },
		"security.ip_blocking.suspicious_per_hour_threshold": func(c *Config, v int) { c.Security.IPBlocking.SuspiciousPerHourThreshold = v },
	}
	for key, set := range fields {
		for _, bad := range []int{0, -1} {
			c := valid()
			set(c, bad)
			if err := c.ValidateSecurityMonitoring(); err == nil || !strings.Contains(err.Error(), key) {
				t.Errorf("%s = %d: err = %v, want the key named", key, bad, err)
			}
			// The numbers are checked with blocking off too: a typo is caught before it is turned on.
			c.Security.IPBlocking.Enabled = false
			if err := c.ValidateSecurityMonitoring(); err == nil || !strings.Contains(err.Error(), key) {
				t.Errorf("%s = %d with blocking off: err = %v, want the key named", key, bad, err)
			}
		}
	}
}
