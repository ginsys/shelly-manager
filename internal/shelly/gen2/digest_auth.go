package gen2

import (
	"crypto/md5" // #nosec G501 -- older firmware still asks for MD5 digest (RFC 2617)
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// digestAuth implements HTTP Digest Authentication: MD5 (RFC 2617) and SHA-256 (RFC 7616), which
// is what Shelly Gen2 devices ask for ("algorithm=SHA-256").
type digestAuth struct {
	username string
	password string
	realm    string
	nonce    string
	qop      string
	opaque   string
	// algorithm is the challenge's algorithm as sent: "" or "MD5" hash with MD5, "SHA-256" with SHA-256.
	algorithm string
	nc        int
	cnonce    string
}

// newDigestAuth creates a new digest auth handler
func newDigestAuth(username, password string) *digestAuth {
	return &digestAuth{
		username: username,
		password: password,
		nc:       0,
	}
}

// parseChallenge parses the WWW-Authenticate header challenge
func (d *digestAuth) parseChallenge(challenge string) error {
	if !strings.HasPrefix(challenge, "Digest ") {
		return fmt.Errorf("not a digest challenge")
	}

	challenge = strings.TrimPrefix(challenge, "Digest ")
	parts := strings.Split(challenge, ",")

	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := kv[0]
		value := strings.Trim(kv[1], `"`)

		switch key {
		case "realm":
			d.realm = value
		case "nonce":
			d.nonce = value
		case "qop":
			d.qop = value
		case "opaque":
			d.opaque = value
		case "algorithm":
			d.algorithm = value
		}
	}

	if d.realm == "" || d.nonce == "" {
		return fmt.Errorf("incomplete digest challenge")
	}

	switch strings.ToUpper(d.algorithm) {
	case "", "MD5", "SHA-256":
	default:
		return fmt.Errorf("unsupported digest algorithm %q", d.algorithm)
	}

	return nil
}

// generateCnonce generates a client nonce
func (d *digestAuth) generateCnonce() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		// Fallback to time-based randomness if crypto/rand fails
		return hex.EncodeToString([]byte(fmt.Sprintf("%x", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

// hash computes the hash of the challenge's algorithm as lowercase hex.
func (d *digestAuth) hash(data string) string {
	if strings.EqualFold(d.algorithm, "SHA-256") {
		sum := sha256.Sum256([]byte(data))
		return hex.EncodeToString(sum[:])
	}
	sum := md5.Sum([]byte(data)) // #nosec G401 -- see the import
	return hex.EncodeToString(sum[:])
}

// computeResponse computes the digest response
func (d *digestAuth) computeResponse(method, uri string) string {
	d.nc++
	d.cnonce = d.generateCnonce()

	// HA1 = H(username:realm:password)
	ha1 := d.hash(fmt.Sprintf("%s:%s:%s", d.username, d.realm, d.password))

	// HA2 = H(method:uri)
	ha2 := d.hash(fmt.Sprintf("%s:%s", method, uri))

	// Response calculation depends on qop
	var response string
	if d.qop != "" {
		// response = H(HA1:nonce:nc:cnonce:qop:HA2)
		ncStr := fmt.Sprintf("%08x", d.nc)
		response = d.hash(fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			ha1, d.nonce, ncStr, d.cnonce, d.qop, ha2))
	} else {
		// response = H(HA1:nonce:HA2)
		response = d.hash(fmt.Sprintf("%s:%s:%s", ha1, d.nonce, ha2))
	}

	return response
}

// setAuthHeader sets the Authorization header on the request
func (d *digestAuth) setAuthHeader(req *http.Request) {
	uri := req.URL.Path
	if req.URL.RawQuery != "" {
		uri += "?" + req.URL.RawQuery
	}

	response := d.computeResponse(req.Method, uri)

	// Build Authorization header
	auth := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		d.username, d.realm, d.nonce, uri, response)

	if d.qop != "" {
		ncStr := fmt.Sprintf("%08x", d.nc)
		auth += fmt.Sprintf(`, qop=%s, nc=%s, cnonce="%s"`, d.qop, ncStr, d.cnonce)
	}

	if d.opaque != "" {
		auth += fmt.Sprintf(`, opaque="%s"`, d.opaque)
	}

	if d.algorithm != "" {
		auth += ", algorithm=" + d.algorithm
	}

	req.Header.Set("Authorization", auth)
}

// doRequestWithDigestAuth performs an HTTP request with digest authentication
func doRequestWithDigestAuth(client *http.Client, req *http.Request, username, password string) (*http.Response, error) {
	// The first request reads the body, so the retry needs a way to get it again: sending the
	// retry with the drained reader would reach the device with an empty body.
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		return nil, fmt.Errorf("digest authentication needs a request body that can be replayed")
	}

	// First request without auth
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// If not 401, return response as-is
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	// Parse WWW-Authenticate header
	challenge := resp.Header.Get("WWW-Authenticate")
	_ = resp.Body.Close()

	if challenge == "" {
		return nil, fmt.Errorf("no WWW-Authenticate header in 401 response")
	}

	// Create digest auth and parse challenge
	auth := newDigestAuth(username, password)
	if parseErr := auth.parseChallenge(challenge); parseErr != nil {
		return nil, fmt.Errorf("failed to parse auth challenge: %w", parseErr)
	}

	// Clone the request for retry
	var body io.Reader
	if req.GetBody != nil {
		replay, bodyErr := req.GetBody()
		if bodyErr != nil {
			return nil, bodyErr
		}
		body = replay
	}
	req2, err := http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), body)
	if err != nil {
		return nil, err
	}
	req2.Header = req.Header.Clone()

	// Set Authorization header
	auth.setAuthHeader(req2)

	// Retry with auth
	return client.Do(req2)
}
