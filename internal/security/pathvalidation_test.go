package security

import (
	"testing"
	"path/filepath"
	"runtime"
)

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errType error
	}{
		{"valid relative path", "file.txt", false, nil},
		{"valid nested path", "dir/file.txt", false, nil},
		{"empty path", "", true, ErrEmptyPath},
		{"path traversal with ..", "../file.txt", true, ErrPathTraversal},
		{"path traversal in middle", "dir/../file.txt", true, ErrPathTraversal},
		{"absolute path", "/etc/passwd", true, ErrAbsolutePath},
		{"null byte", "file\x00.txt", true, ErrInvalidPath},
		{"just dot", ".", true, ErrPathTraversal},
		{"just double dot", "..", true, ErrPathTraversal},
		{"double dot prefix", "../file", true, ErrPathTraversal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil && err != tt.errType {
				t.Errorf("ValidateFilePath() error = %v, wantErrType %v", err, tt.errType)
			}
		})
	}
}

func TestValidateFilePathInBase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to path separator differences")
	}

	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
		errType error
	}{
		{"valid path in base", "file.txt", "/tmp", false, nil},
		{"valid nested path in base", "dir/file.txt", "/tmp", false, nil},
		{"empty base dir", "file.txt", "", true, nil},
		{"path traversal", "../file.txt", "/tmp", true, ErrPathTraversal},
		{"absolute path", "/etc/passwd", "/tmp", true, ErrAbsolutePath},
		{"path outside base", "../../etc/passwd", "/tmp", true, ErrPathTraversal},
		{"empty path", "", "/tmp", true, ErrEmptyPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePathInBase(tt.path, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePathInBase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil && err != tt.errType {
				t.Errorf("ValidateFilePathInBase() error = %v, wantErrType %v", err, tt.errType)
			}
		})
	}
}

func TestValidateStorageKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
		errType error
	}{
		{"valid key", "dir/file.txt", false, nil},
		{"valid single file", "file.txt", false, nil},
		{"empty key", "", true, ErrEmptyPath},
		{"path traversal", "../file.txt", true, ErrPathTraversal},
		{"path traversal in middle", "dir/../file.txt", true, ErrPathTraversal},
		{"absolute path", "/etc/passwd", true, ErrAbsolutePath},
		{"null byte", "file\x00.txt", true, ErrInvalidPath},
		{"dot component", "dir/./file.txt", true, ErrPathTraversal},
		{"double dot component", "dir/../file.txt", true, ErrPathTraversal},
		{"empty component", "dir//file.txt", true, ErrPathTraversal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStorageKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStorageKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil && err != tt.errType {
				t.Errorf("ValidateStorageKey() error = %v, wantErrType %v", err, tt.errType)
			}
		})
	}
}

func TestValidateAndCleanFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
		wantErr  bool
	}{
		{"valid filename", "file.txt", "file.txt", false},
		{"filename with spaces", " file.txt ", "file.txt", false},
		{"empty filename", "", "", true},
		{"only spaces", "   ", "", true},
		{"with forward slash", "dir/file.txt", "", true},
		{"with backslash", "dir\\file.txt", "", true},
		{"with path traversal", "../file.txt", "", true},
		{"with null byte", "file\x00.txt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAndCleanFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndCleanFilename() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateAndCleanFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateExistingFilePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to path separator differences")
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute path", "/tmp/file.txt", false},
		{"valid absolute path with dirs", "/tmp/dir/file.txt", false},
		{"empty path", "", true},
		{"null byte", "/tmp/file\x00.txt", true},
		{"path with traversal", "/tmp/../etc/passwd", true},
		{"relative path", "file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExistingFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExistingFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSafeJoin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows due to path separator differences")
	}

	tests := []struct {
		name     string
		basePath string
		userPath string
		wantErr  bool
	}{
		{"valid join", "/tmp", "file.txt", false},
		{"valid nested join", "/tmp", "dir/file.txt", false},
		{"path traversal", "/tmp", "../file.txt", true},
		{"absolute user path", "/tmp", "/etc/passwd", true},
		{"empty user path", "/tmp", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeJoin(tt.basePath, tt.userPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeJoin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				expected := filepath.Join(tt.basePath, tt.userPath)
				if got != expected {
					t.Errorf("SafeJoin() = %v, want %v", got, expected)
				}
			}
		})
	}
}