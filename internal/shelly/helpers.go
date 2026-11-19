package shelly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DetectGeneration detects the generation of a Shelly device at the given IP
func DetectGeneration(ctx context.Context, ip string) (int, error) {
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		// A request goes to the device address it was given and nowhere else: a redirect
		// is returned as a response, never followed.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// Try Gen2+ RPC endpoint first
	gen2URL := fmt.Sprintf("http://%s/rpc/Shelly.GetDeviceInfo", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", gen2URL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := httpClient.Do(req)
	if err == nil {
		// Close the body whatever the status: a redirect or an error answer is returned as is,
		// and the fallback below overwrites resp.
		var info struct {
			ID  string `json:"id"`
			Gen int    `json:"gen"`
		}
		decodeErr := fmt.Errorf("status %d", resp.StatusCode)
		if resp.StatusCode == http.StatusOK {
			decodeErr = json.NewDecoder(resp.Body).Decode(&info)
		}
		_ = resp.Body.Close()
		if decodeErr == nil && info.Gen > 0 {
			return info.Gen, nil
		}
	}

	// Try Gen1 endpoint
	gen1URL := fmt.Sprintf("http://%s/shelly", ip)
	req, err = http.NewRequestWithContext(ctx, "GET", gen1URL, nil)
	if err != nil {
		return 0, err
	}

	resp, err = httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("device not reachable at %s: %w", ip, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		var info struct {
			Type string `json:"type"`
			MAC  string `json:"mac"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&info); err == nil && info.Type != "" {
			return 1, nil
		}
	}

	return 0, ErrInvalidGeneration
}
