package provider

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

// ValidateConfig checks administrator-supplied endpoints without echoing credentials.
func ValidateConfig(c Config) error {
	for _, raw := range []string{c.OCRURL, c.AIBaseURL, c.CardURL} {
		if raw == "" {
			continue
		}
		if len(raw) > 2048 || strings.TrimSpace(raw) != raw {
			return errors.New("provider URL is invalid")
		}
		u, e := url.Parse(raw)
		if e != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return errors.New("provider URL must be an absolute endpoint without credentials, query, or fragment")
		}
		if u.Scheme == "http" {
			ip := net.ParseIP(u.Hostname())
			if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
				return errors.New("provider URLs require HTTPS except loopback development gateways")
			}
		}
	}
	if len(c.AIModel) > 200 || strings.IndexFunc(c.AIModel, unicode.IsControl) >= 0 {
		return errors.New("AI model name invalid")
	}
	return nil
}

type ConnectionResult struct {
	OK         bool
	StatusCode int
	Message    string
}

// TestConnection checks reachability/auth only; never uploads papers or asks inference.
func (c *Client) TestConnection(ctx context.Context, kind string) (ConnectionResult, error) {
	endpoint, key, method := c.cfg.OCRURL, c.cfg.OCRKey, http.MethodHead
	switch kind {
	case "ocr":
	case "card":
		endpoint, key = c.cfg.CardURL, c.cfg.CardKey
	case "ai":
		endpoint, key, method = strings.TrimRight(c.cfg.AIBaseURL, "/")+"/models", c.cfg.AIKey, http.MethodGet
		if c.cfg.AIBaseURL == "" {
			endpoint = ""
		}
	default:
		return ConnectionResult{}, errors.New("unknown provider")
	}
	if endpoint == "" {
		return ConnectionResult{}, ErrNotConfigured
	}
	check := Config{}
	if kind == "ai" {
		check.AIBaseURL = c.cfg.AIBaseURL
	} else {
		check.OCRURL = endpoint
	}
	if e := ValidateConfig(check); e != nil {
		return ConnectionResult{}, errors.New("provider endpoint invalid")
	}
	req, e := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if e != nil {
		return ConnectionResult{}, errors.New("provider test request invalid")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Accept", "application/json")
	res, e := c.http.Do(req)
	if e != nil {
		return ConnectionResult{}, errors.New("provider connection failed")
	}
	defer res.Body.Close()
	result := ConnectionResult{StatusCode: res.StatusCode}
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		result.OK = true
		result.Message = "endpoint reachable; protocol and recognition accuracy not verified"
	} else if res.StatusCode == http.StatusMethodNotAllowed {
		result.Message = "endpoint reachable but does not support the safe connectivity probe; inference was not attempted"
	} else {
		result.Message = "endpoint returned a non-success status; check URL and credentials"
	}
	return result, nil
}
