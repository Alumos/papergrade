package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ImageType validates compressed bounds without allocating a decoded full-size image.
func ImageType(raw []byte) (string, error) {
	if len(raw) == 0 || len(raw) > 4<<20 {
		return "", errors.New("image exceeds 4MiB limit")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(raw))
	if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8000000 || cfg.Height > 8000000 || int64(cfg.Width)*int64(cfg.Height) > 8000000 {
		return "", errors.New("invalid image or dimensions exceed 8MP")
	}
	switch format {
	case "jpeg":
		return "image/jpeg", nil
	case "png":
		return "image/png", nil
	case "webp":
		return "image/webp", nil
	}
	return "", errors.New("JPEG, PNG or WebP required")
}
func (c *Client) OCRKind() string {
	if c.cfg.OCRType == "ollama" {
		return "ollama"
	}
	return "gateway"
}
func (c *Client) extractGatewayImage(ctx context.Context, raw []byte) (Extraction, error) {
	kind, e := ImageType(raw)
	if e != nil {
		return Extraction{}, e
	}
	if c.cfg.OCRURL == "" {
		return Extraction{}, ErrNotConfigured
	}
	var out Extraction
	if e = c.request(ctx, c.cfg.OCRURL, c.cfg.OCRKey, kind, raw, &out); e != nil {
		return Extraction{}, e
	}
	if len(out.Text) > maxText || strings.TrimSpace(out.Text) == "" {
		return Extraction{}, errors.New("OCR returned empty or oversized text")
	}
	out.Pages = 1
	out.Source = "ocr_gateway"
	return out, nil
}

// Ollama's native API consumes image bytes, not PDF files. Server never rasterizes PDFs.
func (c *Client) extractOllama(ctx context.Context, raw []byte) (Extraction, error) {
	if c.cfg.OCRURL == "" || c.cfg.OCRModel == "" {
		return Extraction{}, ErrNotConfigured
	}
	if _, e := ImageType(raw); e != nil {
		return Extraction{}, e
	}
	if e := ValidateConfig(c.cfg); e != nil {
		return Extraction{}, errors.New("invalid Ollama configuration")
	}
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	body, e := json.Marshal(map[string]any{"model": c.cfg.OCRModel, "think": false, "stream": false, "messages": []map[string]any{{"role": "user", "content": "Extract all information from the main body of the document image and represent it in markdown format, ignoring headers and footers. Tables should be expressed in OTSL format, formulas in the document should be represented using LATEX format, and the parsing should be organized according to the reading order. Only transcribe; do not solve questions, invent answers or guess marks.", "images": []string{base64.StdEncoding.EncodeToString(raw)}}}, "options": map[string]any{"temperature": 0, "num_ctx": 8192, "num_predict": 4096}})
	if e != nil {
		return Extraction{}, errors.New("OCR request encoding failed")
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Done       bool   `json:"done"`
		DoneReason string `json:"done_reason"`
		Error      string `json:"error"`
	}
	ocrClient := *c
	httpClient := *c.http
	httpClient.Timeout = 300 * time.Second
	httpClient.Transport = remoteOllamaTransport
	if ollamaPrivate(c.cfg) {
		httpClient.Transport = directOllamaTransport
	}
	ocrClient.http = &httpClient
	if e = ocrClient.request(ctx, strings.TrimRight(c.cfg.OCRURL, "/")+"/api/chat", c.cfg.OCRKey, "application/json", body, &out); e != nil {
		return Extraction{}, e
	}
	if out.Error != "" || !out.Done || out.DoneReason == "length" || out.DoneReason == "max_tokens" || strings.TrimSpace(out.Message.Content) == "" || len(out.Message.Content) > maxText {
		return Extraction{}, errors.New("Ollama returned incomplete, empty or oversized OCR")
	}
	return Extraction{Text: out.Message.Content, Pages: 1, Source: "ollama"}, nil
}

// Only explicitly selected private literal-address Ollama bypasses environment proxies.
func ollamaPrivate(c Config) bool {
	if c.OCRType != "ollama" {
		return false
	}
	u, e := url.Parse(c.OCRURL)
	if e != nil {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || isCGNAT(ip))
}

func nativeOllamaTransport(direct bool) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if direct {
		t.Proxy = nil
	}
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	t.TLSHandshakeTimeout = 10 * time.Second
	// stream:false may produce no headers until inference completes. Keep this near the total OCR bound, not a short liveness timeout.
	t.ResponseHeaderTimeout = 295 * time.Second
	t.MaxIdleConns = 2
	t.MaxIdleConnsPerHost = 1
	t.IdleConnTimeout = 30 * time.Second
	return t
}

var directOllamaTransport = nativeOllamaTransport(true)
var remoteOllamaTransport = nativeOllamaTransport(false)
