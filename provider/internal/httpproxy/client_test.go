package httpproxy

import (
	"strings"
	"testing"
)

func TestClientEmpty(t *testing.T) {
	c, err := Client("")
	if err != nil || c != nil {
		t.Fatalf("want nil client, got client=%v err=%v", c, err)
	}
}

func TestClientInvalidURL(t *testing.T) {
	_, err := Client("://bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientUnsupportedScheme(t *testing.T) {
	_, err := Client("ftp://127.0.0.1:21")
	if err == nil || !strings.Contains(err.Error(), "unsupported scheme") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}

func TestClientSOCKS5(t *testing.T) {
	c, err := Client("socks5://127.0.0.1:1080")
	if err != nil || c == nil || c.Transport == nil {
		t.Fatalf("client: %v err=%v", c, err)
	}
}

func TestClientOK(t *testing.T) {
	c, err := Client("http://127.0.0.1:7890")
	if err != nil || c == nil || c.Transport == nil {
		t.Fatalf("client: %v err=%v", c, err)
	}
}
