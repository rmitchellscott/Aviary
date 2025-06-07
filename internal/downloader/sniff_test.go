package downloader

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestSniffMimeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	old := sniffTimeout
	SetSniffTimeout(100 * time.Millisecond)
	defer SetSniffTimeout(old)

	_, err := SniffMime(srv.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	var uerr *url.Error
	if errors.As(err, &uerr) {
		if !uerr.Timeout() {
			t.Fatalf("expected timeout, got %v", err)
		}
	} else {
		t.Fatalf("expected url.Error, got %v", err)
	}
}
