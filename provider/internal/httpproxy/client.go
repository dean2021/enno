package httpproxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
)

// Client returns an HTTP client that sends requests through the given proxy URL.
// Supported schemes: http, https (HTTP proxy), socks5, socks5h (SOCKS5; socks5h resolves hostnames via the proxy).
// Empty proxyURL returns (nil, nil): callers should omit the provider SDK HTTP client option.
func Client(proxyURL string) (*http.Client, error) {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return nil, nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("http_proxy: invalid URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return clientHTTPProxy(u)
	case "socks5", "socks5h":
		return clientSOCKSProxy(u)
	default:
		return nil, fmt.Errorf("http_proxy: unsupported scheme %q (use http, https, socks5, or socks5h)", u.Scheme)
	}
}

func clientHTTPProxy(u *url.URL) (*http.Client, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("http_proxy: default transport is not *http.Transport")
	}
	tr := base.Clone()
	tr.Proxy = http.ProxyURL(u)
	return &http.Client{Transport: tr}, nil
}

func clientSOCKSProxy(u *url.URL) (*http.Client, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("http_proxy: default transport is not *http.Transport")
	}
	dialer, err := proxy.FromURL(u, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("http_proxy: socks dialer: %w", err)
	}
	tr := base.Clone()
	tr.Proxy = nil
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			return cd.DialContext(ctx, network, addr)
		}
		return dialer.Dial(network, addr)
	}
	return &http.Client{Transport: tr}, nil
}
