package prober

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Ar52015/beacon/internal/store"
)

// interface
type Prober interface {
	Probe(ctx context.Context) store.Result
}

// prober types
type HTTPProber struct {
	URL string
}

type TCPProber struct {
	Address string
}

type DNSProber struct {
	Host string
}

type TLSProber struct {
	Address string
}

type JSONRPCProber struct {
	URL string
}

// probes
func (p *HTTPProber) Probe(ctx context.Context) store.Result {
	// start recording
	start := time.Now()

	// probe the target
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		slog.Debug("Failed to issue request object", "err", err)
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     err.Error(),
		}
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		slog.Debug("Failed to issue recieve response", "err", err)
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     err.Error(),
		}
	}
	defer func() { _ = resp.Body.Close() }()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		slog.Debug("Failed to discard response body", "err", err)
	}

	// stop recording & return
	elapsed := time.Since(start).Milliseconds()

	return store.Result{
		Timestamp: start,
		LatencyMs: int(elapsed),
		OK:        ok,
		Error:     "",
	}
}

func (p *TCPProber) Probe(ctx context.Context) store.Result {
	// start recording
	start := time.Now()

	// Probe
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", p.Address)
	if err != nil {
		slog.Debug("Failed to issue a TCP connection", "err", err)
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     err.Error(),
		}
	}
	defer func() { _ = conn.Close() }()

	// stop recording
	elapsed := int(time.Since(start).Milliseconds())

	return store.Result{
		Timestamp: start,
		LatencyMs: elapsed,
		OK:        true,
		Error:     "",
	}
}

func (p *DNSProber) Probe(ctx context.Context) store.Result {
	// start recording
	start := time.Now()

	// probe
	var r net.Resolver
	addrs, err := r.LookupHost(ctx, p.Host)
	if err != nil {
		slog.Debug("DNS lookup failed", "err", err)
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     err.Error(),
		}
	}
	if len(addrs) == 0 {
		slog.Debug("lookup returned 0 addresses")
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     "Lookup returned 0 addresses",
		}
	}

	// stop recording
	elapsed := int(time.Since(start).Milliseconds())

	return store.Result{
		Timestamp: start,
		LatencyMs: elapsed,
		OK:        true,
		Error:     "",
	}
}

func (p *TLSProber) Probe(ctx context.Context) store.Result {
	// start recording
	start := time.Now()

	// probe
	var d tls.Dialer
	conn, err := d.DialContext(ctx, "tcp", p.Address)
	if err != nil {
		slog.Debug("TLS handshake failed", "err", err)
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     err.Error(),
		}
	}
	defer func() { _ = conn.Close() }()

	state := conn.(*tls.Conn).ConnectionState()
	if len(state.PeerCertificates) == 0 {
		slog.Debug("no peer certificates")
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     "no peer certificates",
		}
	}
	leaf := state.PeerCertificates[0]
	days := int(time.Until(leaf.NotAfter).Hours() / 24)

	// stop recording
	elapsed := int(time.Since(start).Milliseconds())

	return store.Result{
		Timestamp: start,
		LatencyMs: elapsed,
		OK:        true,
		Error:     "",
		Info:      fmt.Sprintf("Cert Expires in %d days", days),
	}
}

func (p *JSONRPCProber) Probe(ctx context.Context) store.Result {
	// start recording
	start := time.Now()

	// probe
	const body = `{"jsonrpc":"2.0","method":"eth_blockNumber","id":1}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, strings.NewReader(body))
	if err != nil {
		slog.Debug("failed to issue JSONRPC request", "err", err)
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     err.Error(),
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		slog.Debug("failed to recieve response", "err", err)
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     err.Error(),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Result *json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		slog.Debug("failed to decode json body", "err", err)
		return store.Result{
			Timestamp: start,
			LatencyMs: int(time.Since(start).Milliseconds()),
			OK:        false,
			Error:     err.Error(),
		}
	}

	// stop recording
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		slog.Debug("Failed to discard response body", "err", err)
	}
	elapsed := int(time.Since(start).Milliseconds())

	if out.Result == nil {
		msg := "no result in response"
		if out.Error != nil {
			msg = fmt.Sprintf("rpc error %d: %s", out.Error.Code, out.Error.Message)
		}
		return store.Result{
			Timestamp: start,
			LatencyMs: elapsed,
			OK:        false,
			Error:     msg,
		}
	}

	return store.Result{
		Timestamp: start,
		LatencyMs: elapsed,
		OK:        true,
		Error:     "",
	}
}

// Probe Router
func New(t store.Target) (Prober, error) {
	switch t.Kind {
	case store.KindHTTP:
		return &HTTPProber{URL: t.URL}, nil
	case store.KindTCP:
		return &TCPProber{Address: t.URL}, nil
	case store.KindDNS:
		return &DNSProber{Host: t.URL}, nil
	case store.KindTLS:
		return &TLSProber{Address: t.URL}, nil
	case store.KindJSONRPC:
		return &JSONRPCProber{URL: t.URL}, nil
	default:
		return nil, fmt.Errorf("unknown prober kind: %q", t.Kind)
	}
}
