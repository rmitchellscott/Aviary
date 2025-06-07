package webhook

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestIsTrue(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"no", false},
		{"0", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isTrue(c.in); got != c.want {
			t.Errorf("isTrue(%q)=%v, want %v", c.in, got, c.want)
		}
	}
}

func TestGetFileHeader(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("hello"))
	writer.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatal(err)
	}

	fh, err := getFileHeader(req, "file")
	if err != nil {
		t.Fatalf("getFileHeader returned error: %v", err)
	}
	if fh.Filename != "test.txt" {
		t.Errorf("expected filename test.txt, got %s", fh.Filename)
	}

	if _, err := getFileHeader(req, "missing"); err == nil {
		t.Errorf("expected error for missing field")
	}
}
