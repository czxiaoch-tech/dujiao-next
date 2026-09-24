package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var _, carrierGradeNAT, _ = net.ParseCIDR("100.64.0.0/10")

// ValidatePublicHTTPURL rejects server-side HTTP targets that resolve to
// localhost, private, link-local, multicast, unspecified, or CGNAT addresses.
func ValidatePublicHTTPURL(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid outbound url: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("outbound url must use http or https")
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(parsed.Hostname())), ".")
	if host == "" {
		return fmt.Errorf("outbound url host is required")
	}
	_, err = resolvePublicIPs(ctx, host)
	return err
}

func resolvePublicIPs(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return nil, fmt.Errorf("outbound host is required")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, fmt.Errorf("local outbound address is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("private or local outbound address is not allowed")
		}
		return []net.IP{ip}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve outbound host: %w", err)
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("outbound host has no resolved address")
	}
	ips := make([]net.IP, 0, len(resolved))
	for _, address := range resolved {
		ip := address.IP
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("private or local outbound address is not allowed")
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast() ||
		(carrierGradeNAT != nil && carrierGradeNAT.Contains(ip))
}

type guardedTransport struct {
	base http.RoundTripper
}

func (t guardedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("invalid outbound request")
	}
	if err := ValidatePublicHTTPURL(req.Context(), req.URL.String()); err != nil {
		return nil, err
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// NewHTTPClient returns an HTTP client that checks both the requested URL and
// every redirect before sending traffic.
func NewHTTPClient(timeout time.Duration) *http.Client {
	base := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := resolvePublicIPs(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("outbound host has no dialable address")
		}
		return nil, lastErr
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: guardedTransport{base: base},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if req == nil || req.URL == nil {
			return fmt.Errorf("invalid outbound redirect")
		}
		return ValidatePublicHTTPURL(req.Context(), req.URL.String())
	}
	return client
}
