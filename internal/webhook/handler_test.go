package webhook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rmitchellscott/aviary/internal/compressor"
	"github.com/rmitchellscott/aviary/internal/manager"
)

// stubCommand records commands and returns a Cmd that succeeds
func stubCommand(records *[][]string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		*records = append(*records, append([]string{name}, args...))
		return exec.Command("true")
	}
}

func TestProcessPDFRmapiCommands(t *testing.T) {
	pdfDir := t.TempDir()
	oldPdfDir := os.Getenv("PDF_DIR")
	os.Setenv("PDF_DIR", pdfDir)
	defer os.Setenv("PDF_DIR", oldPdfDir)

	rmDir := "/Books"

	combinations := []struct {
		compress bool
		manage   bool
		archive  bool
	}{
		{false, false, false},
		{true, false, false},
		{false, false, true},
		{true, false, true},
		{false, true, false},
		{true, true, false},
		{false, true, true},
		{true, true, true},
	}

	for _, c := range combinations {
		name := fmt.Sprintf("compress_%t_manage_%t_archive_%t", c.compress, c.manage, c.archive)
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			input := filepath.Join(tmpDir, "input.pdf")
			if err := os.WriteFile(input, []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}
			if c.compress {
				compPath := strings.TrimSuffix(input, ".pdf") + "_compressed.pdf"
				if err := os.WriteFile(compPath, []byte("comp"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			var cmds [][]string
			mgrOrig := manager.ExecCommand
			compOrig := compressor.ExecCommand
			manager.ExecCommand = stubCommand(&cmds)
			compressor.ExecCommand = func(string, ...string) *exec.Cmd { return exec.Command("true") }
			defer func() {
				manager.ExecCommand = mgrOrig
				compressor.ExecCommand = compOrig
			}()

			form := map[string]string{
				"Body":           input,
				"prefix":         "Report",
				"compress":       strconv.FormatBool(c.compress),
				"manage":         strconv.FormatBool(c.manage),
				"archive":        strconv.FormatBool(c.archive),
				"rm_dir":         rmDir,
				"retention_days": "7",
			}
			if _, err := processPDF(form); err != nil {
				t.Fatalf("processPDF error: %v", err)
			}

			today := time.Now()
			month := today.Format("January")
			day := today.Day()
			var expect [][]string
			if c.manage && c.archive {
				dest := filepath.Join(pdfDir, "Report", fmt.Sprintf("Report %s %d.pdf", month, day))
				expect = append(expect, []string{"rmapi", "put", dest, rmDir})
				expect = append(expect, []string{"rmapi", "ls", rmDir})
			} else if c.manage && !c.archive {
				dest := filepath.Join(tmpDir, fmt.Sprintf("Report %s %d.pdf", month, day))
				expect = append(expect, []string{"rmapi", "put", dest, rmDir})
				expect = append(expect, []string{"rmapi", "ls", rmDir})
			} else {
				expect = append(expect, []string{"rmapi", "put", input, rmDir})
				if c.manage {
					expect = append(expect, []string{"rmapi", "ls", rmDir})
				}
			}

			if !reflect.DeepEqual(cmds, expect) {
				t.Fatalf("commands mismatch:\n got  %v\n want %v", cmds, expect)
			}
		})
	}
}
