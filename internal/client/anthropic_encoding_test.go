package client

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

func quietLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// The regression this guards: a client (n8n's Node/undici, curl --compressed, ...)
// sends "Accept-Encoding: gzip, deflate, br"; the proxy used to forward that
// verbatim, Anthropic answered in brotli, and the proxy — which only inflates
// gzip — returned the raw brotli bytes with the Content-Encoding header stripped.
func TestForwardRequestPinsAcceptEncoding(t *testing.T) {
	cases := map[string]string{
		"node-undici default": "gzip, deflate, br",
		"brotli only":         "br",
		"identity":            "identity",
		"absent":              "",
	}
	for name, clientAE := range cases {
		t.Run(name, func(t *testing.T) {
			var seen string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.Header.Get("Accept-Encoding")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}]}`))
			}))
			defer srv.Close()

			pc := NewProxyClient(srv.URL, quietLogger())
			headers := map[string]string{"x-api-key": "sk-test"}
			if clientAE != "" {
				headers["Accept-Encoding"] = clientAE
			}
			resp, err := pc.ForwardRequest(&types.AnthropicRequest{Model: "claude-test", MaxTokens: 1}, headers)
			if err != nil {
				t.Fatalf("ForwardRequest: %v", err)
			}
			resp.Body.Close()
			if seen != upstreamAcceptEncoding {
				t.Fatalf("upstream saw Accept-Encoding %q, want %q (client sent %q)", seen, upstreamAcceptEncoding, clientAE)
			}
		})
	}
}

func TestGetModelsPinsAcceptEncoding(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Accept-Encoding")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	pc := NewProxyClient(srv.URL, quietLogger())
	resp, err := pc.GetModels(map[string]string{"Accept-Encoding": "br"})
	if err != nil {
		t.Fatalf("GetModels: %v", err)
	}
	resp.Body.Close()
	if seen != upstreamAcceptEncoding {
		t.Fatalf("upstream saw Accept-Encoding %q, want %q", seen, upstreamAcceptEncoding)
	}
}

func gzipBytes(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func fakeResponse(encoding string, body []byte) *http.Response {
	h := http.Header{}
	if encoding != "" {
		h.Set("Content-Encoding", encoding)
	}
	return &http.Response{StatusCode: http.StatusOK, Header: h, Body: io.NopCloser(bytes.NewReader(body))}
}

const okJSON = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"BROTLI-TEST-OK"}]}`

func TestReadAndParseResponseInflatesGzip(t *testing.T) {
	pc := NewProxyClient("http://unused", quietLogger())
	parsed, body, err := pc.ReadAndParseResponse(fakeResponse("gzip", gzipBytes(t, okJSON)))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if parsed.ID != "msg_1" || !strings.Contains(string(body), "BROTLI-TEST-OK") {
		t.Fatalf("gzip body not inflated/parsed: %q", body)
	}
}

func TestReadAndParseResponseAcceptsIdentity(t *testing.T) {
	pc := NewProxyClient("http://unused", quietLogger())
	for _, enc := range []string{"", "identity"} {
		parsed, _, err := pc.ReadAndParseResponse(fakeResponse(enc, []byte(okJSON)))
		if err != nil {
			t.Fatalf("encoding %q: %v", enc, err)
		}
		if parsed.ID != "msg_1" {
			t.Fatalf("encoding %q: parsed id %q", enc, parsed.ID)
		}
	}
}

func TestReadAndParseResponseRefusesUnsupportedEncoding(t *testing.T) {
	pc := NewProxyClient("http://unused", quietLogger())
	raw := []byte{0x1b, 0x2f, 0x00, 0xc4, 0x7f} // arbitrary "brotli-looking" bytes
	parsed, body, err := pc.ReadAndParseResponse(fakeResponse("br", raw))
	if err == nil {
		t.Fatal("expected an error for Content-Encoding: br, got nil")
	}
	if parsed != nil {
		t.Fatal("expected no parsed response for undecodable body")
	}
	if !strings.Contains(err.Error(), `unsupported Content-Encoding "br"`) {
		t.Fatalf("error should name the encoding, got: %v", err)
	}
	if !bytes.Equal(body, raw) {
		t.Fatalf("raw body must be returned untouched so the caller can forward it labelled; got %v", body)
	}
}
