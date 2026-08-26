package security

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func newTestClient() *http.Client {
	client := NewHTTPClient()
	client.Timeout = 5 * time.Second
	return client
}

func TestRedirectToLinkLocalRefused(t *testing.T) {
	os.Setenv("BLOCK_PRIVATE_IPS", "false")
	defer os.Unsetenv("BLOCK_PRIVATE_IPS")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer server.Close()

	_, err := newTestClient().Get(server.URL)
	if err == nil {
		t.Fatal("expected redirect to link-local address to be refused")
	}
	if !errors.Is(err, ErrLinkLocal) {
		t.Errorf("expected ErrLinkLocal, got %v", err)
	}
}

func TestDialToLinkLocalRefused(t *testing.T) {
	os.Unsetenv("BLOCK_PRIVATE_IPS")

	_, err := newTestClient().Get("http://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Fatal("expected link-local address to be refused at dial time")
	}
	if !errors.Is(err, ErrLinkLocal) {
		t.Errorf("expected ErrLinkLocal, got %v", err)
	}
}

func TestRedirectLimit(t *testing.T) {
	os.Setenv("BLOCK_PRIVATE_IPS", "false")
	defer os.Unsetenv("BLOCK_PRIVATE_IPS")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/again", http.StatusFound)
	}))
	defer server.Close()

	_, err := newTestClient().Get(server.URL)
	if !errors.Is(err, ErrTooManyRedirects) {
		t.Errorf("expected ErrTooManyRedirects, got %v", err)
	}
}

func TestLinkLocalBlockedRegardlessOfFlag(t *testing.T) {
	os.Unsetenv("BLOCK_PRIVATE_IPS")
	defer os.Unsetenv("BLOCK_PRIVATE_IPS")

	for _, rawURL := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.170.2/v2/credentials",
		"http://[fe80::1]/",
	} {
		if err := ValidateURL(rawURL); !errors.Is(err, ErrLinkLocal) {
			t.Errorf("ValidateURL(%s) = %v, want ErrLinkLocal", rawURL, err)
		}
	}
}

func TestPrivateAddressesBlockedByDefault(t *testing.T) {
	os.Unsetenv("BLOCK_PRIVATE_IPS")

	for _, rawURL := range []string{
		"http://192.168.1.50/doc.pdf",
		"http://10.0.0.5/doc.pdf",
		"http://127.0.0.1:8080/doc.pdf",
		"http://100.64.0.1/doc.pdf",
	} {
		if err := ValidateURL(rawURL); !errors.Is(err, ErrPrivateIP) {
			t.Errorf("ValidateURL(%s) = %v, want ErrPrivateIP", rawURL, err)
		}
	}
}

func TestPrivateAddressesAllowedWhenOptedOut(t *testing.T) {
	os.Setenv("BLOCK_PRIVATE_IPS", "false")
	defer os.Unsetenv("BLOCK_PRIVATE_IPS")

	for _, rawURL := range []string{
		"http://192.168.1.50/doc.pdf",
		"http://10.0.0.5/doc.pdf",
		"http://127.0.0.1:8080/doc.pdf",
		"http://100.64.0.1/doc.pdf",
	} {
		if err := ValidateURL(rawURL); err != nil {
			t.Errorf("ValidateURL(%s) = %v, want nil", rawURL, err)
		}
	}
}
