package server

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"autocache/internal/config"

	"github.com/sirupsen/logrus"
)

func newEncodingTestHandler(t *testing.T, upstream string) *AutocacheHandler {
	t.Helper()
	cfg := &config.Config{AnthropicURL: upstream, CacheStrategy: "moderate"}
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	return NewAutocacheHandler(cfg, logger, "1.0.0-test")
}

// writeRawResponse is the error path that used to ship header-less brotli:
// Content-Encoding must be dropped only for gzip (the body was inflated),
// and kept for anything else (the body is still exactly what upstream sent).
func TestWriteRawResponseContentEncodingRule(t *testing.T) {
	h := newEncodingTestHandler(t, "http://unused")

	cases := []struct {
		encoding string
		keep     bool
	}{
		{"gzip", false},
		{"GZIP", false},
		{"br", true},
		{"zstd", true},
		{"identity", true},
	}
	for _, c := range cases {
		t.Run(c.encoding, func(t *testing.T) {
			rec := httptest.NewRecorder()
			hdr := http.Header{}
			hdr.Set("Content-Encoding", c.encoding)
			hdr.Set("X-Passthrough", "yes")
			h.writeRawResponse(rec, http.StatusBadGateway, []byte("body"), hdr)

			got := rec.Header().Get("Content-Encoding")
			if c.keep && got == "" {
				t.Fatalf("Content-Encoding %q must be preserved on an undecoded body", c.encoding)
			}
			if !c.keep && got != "" {
				t.Fatalf("Content-Encoding %q must be dropped on an inflated body, got %q", c.encoding, got)
			}
			if rec.Header().Get("X-Passthrough") != "yes" {
				t.Fatal("other headers must still be copied")
			}
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status %d, want %d", rec.Code, http.StatusBadGateway)
			}
		})
	}
}

// End to end through the handler: the client asks for brotli, upstream honours
// whatever it is asked, and the client must still get parseable JSON.
func TestHandleMessagesClientBrotliUpstreamGzip(t *testing.T) {
	const reply = `{"id":"msg_e2e","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"BROTLI-TEST-OK"}],"usage":{"input_tokens":1,"output_tokens":1}}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ae := r.Header.Get("Accept-Encoding")
		if strings.Contains(ae, "br") {
			// A faithful upstream would answer in brotli here — which this proxy
			// cannot decode. The pin must prevent us from ever asking for it.
			w.Header().Set("Content-Encoding", "br")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{0x1b, 0x2f, 0x00, 0xc4, 0x7f})
			return
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write([]byte(reply))
		_ = zw.Close()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}))
	defer upstream.Close()

	h := newEncodingTestHandler(t, upstream.URL)

	body := `{"model":"claude-test","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "sk-test")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	rec := httptest.NewRecorder()

	h.HandleMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
	}
	if ce := rec.Header().Get("Content-Encoding"); ce != "" {
		t.Fatalf("inflated body must not carry Content-Encoding, got %q", ce)
	}
	if !strings.Contains(rec.Body.String(), `"BROTLI-TEST-OK"`) {
		t.Fatalf("client did not receive the JSON text, got %q", rec.Body.String())
	}
}
