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
func isCGNAT(ip net.IP) bool {
	v := ip.To4()
	return v != nil && v[0] == 100 && v[1] >= 64 && v[1] <= 127
}
func ValidateConfig(c Config) error {
	if c.OCRType != "" && c.OCRType != "gateway" && c.OCRType != "ollama" {
		return errors.New("OCR type must be gateway or ollama")
	}
	if c.CardMode != "" && c.CardMode != "omr" && c.CardMode != "gateway" {
		return errors.New("card mode must be omr or gateway")
	}
	if len(c.OCRModel) > 200 || strings.IndexFunc(c.OCRModel, unicode.IsControl) >= 0 {
		return errors.New("OCR model name invalid")
	}
	for i, raw := range []string{c.OCRURL, c.AIBaseURL, c.CardURL} {
		if raw == "" {
			continue
		}
		if i == 0 && c.OCRType == "ollama" {
			u, e := url.Parse(raw)
			if e != nil || (u.Path != "" && u.Path != "/") {
				return errors.New("Ollama URL must be a base origin without /api/chat")
			}
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
			privateOllama := i == 0 && c.OCRType == "ollama" && ip != nil && (ip.IsPrivate() || isCGNAT(ip))
			if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) && !privateOllama {
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
		if c.cfg.OCRType == "ollama" {
			endpoint, method = strings.TrimRight(c.cfg.OCRURL, "/")+"/api/tags", http.MethodGet
		}
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
		if kind == "ocr" && c.cfg.OCRType == "ollama" {
			check.OCRURL = c.cfg.OCRURL
		}
		if kind == "ocr" {
			check.OCRType = c.cfg.OCRType
		}
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
	httpClient := c.http
	if kind == "ocr" && ollamaPrivate(c.cfg) {
		copyClient := *c.http
		copyClient.Transport = directOllamaTransport
		httpClient = &copyClient
	}
	res, e := httpClient.Do(req)
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
