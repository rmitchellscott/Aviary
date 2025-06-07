package manager

import (
	"bufio"
	"bytes"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// ListFolders returns a slice of all folder paths on the reMarkable device.
// Paths are returned with a leading slash, e.g. "/Books/Fiction".
func ListFolders() ([]string, error) {
	// Include the root directory explicitly so the UI can offer it as an option
	folders := []string{"/"}

	var walk func(string) error
	walk = func(p string) error {
		args := []string{"ls"}
		if p != "" {
			args = append(args, p)
		}
		out, err := ExecCommand("rmapi", args...).Output()
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "[d]") {
				name := strings.TrimSpace(strings.TrimPrefix(line, "[d]"))
				name = strings.TrimLeft(name, "\t ")

				// Skip the /trash folder entirely
				if p == "" && name == "trash" {
					continue
				}

				var child string
				if p == "" {
					child = name
				} else {
					child = path.Join(p, name)
				}
				folders = append(folders, "/"+child)
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return scanner.Err()
	}

	if err := walk(""); err != nil {
		return nil, err
	}
	return folders, nil
}

// FoldersHandler writes a JSON {"folders": ["/path", ...]}.
func FoldersHandler(c *gin.Context) {
	dirs, err := ListFolders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"folders": dirs})
}
