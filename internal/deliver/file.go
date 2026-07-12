package deliver

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/quangkhaidam93/dev-digest/internal/config"
)

// deliverFile writes the digest to <dir>/<YYYY-MM-DD>.<ext> for each requested
// format.
func deliverFile(cfg config.FileDelivery, date time.Time, md, html string) error {
	dir := cfg.Dir
	if dir == "" {
		dir = "./out"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	base := date.Format("2006-01-02")
	formats := cfg.Formats
	if len(formats) == 0 {
		formats = []string{"md", "html"}
	}

	for _, f := range formats {
		var content, ext string
		switch f {
		case "md":
			content, ext = md, "md"
		case "html":
			content, ext = html, "html"
		default:
			return fmt.Errorf("unknown file format %q", f)
		}
		path := filepath.Join(dir, base+"."+ext)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
