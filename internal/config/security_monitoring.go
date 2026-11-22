package config

import "fmt"

// ValidateSecurityMonitoring stops a start-up whose security monitoring keys cannot do what they say:
// IP blocking is on while the monitor that does the blocking is off, or the block length or a
// threshold is zero or negative. The manager calls it after loading the configuration; the
// provisioner reads the same file and does not call it, because it builds no monitor.
func (c *Config) ValidateSecurityMonitoring() error {
	if c.Security.IPBlocking.Enabled && !c.Security.Monitoring.Enabled {
		return fmt.Errorf("security.ip_blocking.enabled is true while security.monitoring.enabled is false: blocking needs the monitor, so turn on security.monitoring.enabled or turn off security.ip_blocking.enabled")
	}
	for _, key := range []struct {
		name  string
		value int
	}{
		{"security.ip_blocking.block_seconds", c.Security.IPBlocking.BlockSeconds},
		{"security.ip_blocking.suspicious_threshold", c.Security.IPBlocking.SuspiciousThreshold},
		{"security.ip_blocking.rate_limit_threshold", c.Security.IPBlocking.RateLimitThreshold},
		{"security.ip_blocking.attack_type_threshold", c.Security.IPBlocking.AttackTypeThreshold},
		{"security.ip_blocking.suspicious_per_hour_threshold", c.Security.IPBlocking.SuspiciousPerHourThreshold},
	} {
		if key.value <= 0 {
			return fmt.Errorf("%s must be greater than zero, got %d", key.name, key.value)
		}
	}
	return nil
}
