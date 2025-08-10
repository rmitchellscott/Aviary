package security

import (
	"testing"
	"runtime"
)

func TestSecurePath(t *testing.T) {
	t.Run("NewSecurePath", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			wantErr bool
		}{
			{"valid relative path", "file.txt", false},
			{"valid nested path", "dir/file.txt", false},
			{"invalid absolute path", "/etc/passwd", true},
			{"invalid traversal", "../file.txt", true},
			{"invalid null byte", "file\x00.txt", true},
			{"empty path", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sp, err := NewSecurePath(tt.input)
				if (err != nil) != tt.wantErr {
					t.Errorf("NewSecurePath() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && sp == nil {
					t.Errorf("NewSecurePath() returned nil SecurePath for valid input")
				}
				if !tt.wantErr && sp.String() == "" {
					t.Errorf("NewSecurePath() returned empty string for valid input")
				}
			})
		}
	})

	t.Run("NewSecurePathFromExisting", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping on Windows due to path differences")
		}
		
		tests := []struct {
			name    string
			input   string
			wantErr bool
		}{
			{"valid absolute path", "/tmp/file.txt", false},
			{"valid nested absolute", "/tmp/dir/file.txt", false},
			{"invalid relative path", "file.txt", true},
			{"invalid traversal", "/tmp/../etc/passwd", true},
			{"invalid null byte", "/tmp/file\x00.txt", true},
			{"empty path", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sp, err := NewSecurePathFromExisting(tt.input)
				if (err != nil) != tt.wantErr {
					t.Errorf("NewSecurePathFromExisting() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && sp == nil {
					t.Errorf("NewSecurePathFromExisting() returned nil SecurePath for valid input")
				}
			})
		}
	})

	t.Run("SecurePath methods", func(t *testing.T) {
		sp, err := NewSecurePath("dir/file.txt")
		if err != nil {
			t.Fatalf("Failed to create SecurePath: %v", err)
		}

		t.Run("String", func(t *testing.T) {
			str := sp.String()
			if str == "" {
				t.Error("String() returned empty string")
			}
		})

		t.Run("Dir", func(t *testing.T) {
			dir := sp.Dir()
			if dir == nil {
				t.Error("Dir() returned nil")
			}
			if dir.String() == "" {
				t.Error("Dir().String() returned empty string")
			}
		})

		t.Run("Join", func(t *testing.T) {
			joined, err := sp.Join("subfile.txt")
			if err != nil {
				t.Errorf("Join() error = %v", err)
			}
			if joined == nil {
				t.Error("Join() returned nil")
			}
			
			// Test invalid join
			_, err = sp.Join("../malicious.txt")
			if err == nil {
				t.Error("Join() should reject path traversal")
			}
		})
	})

	t.Run("nil SecurePath", func(t *testing.T) {
		var sp *SecurePath

		if sp.String() != "" {
			t.Error("nil SecurePath.String() should return empty string")
		}
		
		if sp.Dir() != nil {
			t.Error("nil SecurePath.Dir() should return nil")
		}
		
		_, err := sp.Join("test")
		if err == nil {
			t.Error("nil SecurePath.Join() should return error")
		}
	})
}