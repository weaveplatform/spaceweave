package dwg

import (
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// TestRealFiles reads every .dwg under $SPACEWEAVE_DWG_DIR, logs what each
// holds and renders it to a PNG (in $SPACEWEAVE_DWG_OUT, or a temporary
// directory) to compare with a plot of the drawing. Third-party drawings
// can't live in the repository, so this runs only when pointed at some.
func TestRealFiles(t *testing.T) {
	dir := os.Getenv("SPACEWEAVE_DWG_DIR")
	if dir == "" {
		t.Skip("set SPACEWEAVE_DWG_DIR to a directory of .dwg files")
	}
	out := os.Getenv("SPACEWEAVE_DWG_OUT")
	if out == "" {
		out = t.TempDir()
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() && strings.EqualFold(filepath.Ext(path), ".dwg") {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path) //nolint:gosec // the files it was pointed at
			if err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			doc, st, err := decode(data)
			took := time.Since(start)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			t.Logf("%s %s, units %s, %v", doc.Version, filepath.Base(path), doc.Units, took)
			t.Logf("model %d, paper %d, layers %d, blocks %d, skipped %d",
				len(doc.Model), len(doc.Paper), len(doc.Layers), len(doc.Blocks), doc.Skipped)
			t.Logf("decoded: %s", counts(st.types))
			t.Logf("skipped: %s", counts(st.skipped))
			t.Logf("notes: %q", doc.Notes)
			render(
				t,
				doc,
				filepath.Join(out, strings.TrimSuffix(filepath.Base(path), ".dwg")+".png"),
			)
		})
	}
}

func counts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

func render(t *testing.T, doc *cad.Document, path string) {
	t.Helper()
	d, err := cad.Draw(doc)
	if err != nil {
		t.Fatalf("draw: %v", err)
	}
	ras, err := vector.Rasterize(d, vector.Options{MaxSide: 2048})
	if err != nil {
		t.Fatalf("rasterise: %v", err)
	}
	f, err := os.Create(path) //nolint:gosec // the test's own output
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, ras.Image); err != nil {
		t.Fatal(err)
	}
	t.Logf("rendered %s (%d notes: %q)", path, len(d.Notes), d.Notes)
}
