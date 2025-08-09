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
	}
	
	// Try Gen2+ RPC endpoint first
	gen2URL := fmt.Sprintf("http://%s/rpc/Shelly.GetDeviceInfo", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", gen2URL, nil)
	if err != nil {
		return 0, err
	}
	
	resp, err := httpClient.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var info struct {
			ID  string `json:"id"`
			Gen int    `json:"gen"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&info); err == nil && info.Gen > 0 {
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
	defer resp.Body.Close()
	
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