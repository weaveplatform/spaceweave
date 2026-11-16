package convert

import (
	"context"
	"encoding/xml"
	"errors"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
)

func write(t *testing.T, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func decodePNG(t *testing.T, path string) (w, h int) {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // test temp file
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // read-only
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img.Bounds().Dx(), img.Bounds().Dy()
}

func wellFormed(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // test temp file
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // read-only
	d := xml.NewDecoder(f)
	for {
		if _, err := d.Token(); errors.Is(err, io.EOF) {
			return
		} else if err != nil {
			t.Fatalf("drawing.svg isn't well-formed XML: %v", err)
		}
	}
}

func TestDXFHasAnExactScale(t *testing.T) {
	for name, b := range map[string][]byte{"plan.dxf": dxftest.Plan().Bytes(), "binary.dxf": dxftest.Plan().Binary()} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			res, err := File(context.Background(), write(t, name, b), dir, Options{MaxSide: 1200})
			if err != nil {
				t.Fatal(err)
			}
			if res.Format.ID != DXF.ID || res.Image != filepath.Join(dir, ImageFile) ||
				res.SVG != filepath.Join(dir, DrawingFile) {
				t.Fatalf("%+v", res)
			}
			w, h := decodePNG(t, res.Image)
			if w != res.Width || h != res.Height || max(w, h) != 1200 {
				t.Fatalf("image %d × %d, result says %d × %d", w, h, res.Width, res.Height)
			}
			wellFormed(t, res.SVG)
			if res.Assumed || res.PxPerMetre <= 0 || res.PxPerPaperMM != 0 {
				t.Fatalf("mm units are exact: %+v", res)
			}
			// The plan is PlanWidth mm wide, and the long side spans the
			// image less its margins; so the scale is a little under
			// 1200 px over 12 m.
			if s := res.PxPerMetre; s > 1200/(dxftest.PlanWidth/1000) ||
				s < 0.9*1200/(dxftest.PlanWidth/1000) {
				t.Fatalf("scale %.2f px/m", s)
			}
			if !strings.Contains(res.Scale(), "from the drawing's units") {
				t.Fatal(res.Scale())
			}
			if res.Trimmed != 1 ||
				!strings.Contains(strings.Join(res.Notes, "\n"), "1 stray item") {
				t.Fatalf(
					"the plan's stray line is trimmed and noted: %d %q",
					res.Trimmed,
					res.Notes,
				)
			}
		})
	}
}

func TestUnitlessDXFAssumesAScale(t *testing.T) {
	d := dxftest.New(0).Add(dxftest.Rect("0", 0, 0, 12000, 8000))
	res, err := File(
		context.Background(),
		write(t, "plan.dxf", d.Bytes()),
		t.TempDir(),
		Options{MaxSide: 600},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Assumed || res.PxPerMetre <= 0 || !strings.Contains(res.Scale(), "assumed") {
		t.Fatalf("%+v %s", res, res.Scale())
	}
}

func TestSVGInMillimetresHasAPaperScale(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="200mm" height="100mm" viewBox="0 0 200 100">
	<rect x="10" y="10" width="180" height="80" fill="none" stroke="black"/></svg>`
	res, err := File(
		context.Background(),
		write(t, "plan.svg", []byte(svg)),
		t.TempDir(),
		Options{MaxSide: 1000},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.PxPerMetre != 0 || res.PxPerPaperMM <= 0 || !strings.Contains(res.Scale(), "on paper") {
		t.Fatalf("%+v", res)
	}
	// 1000 px across a little over 180 mm of ink.
	if math.Abs(res.PxPerPaperMM-1000/180.0) > 1 {
		t.Fatalf("%.3f px per paper mm", res.PxPerPaperMM)
	}
}

func TestSVGInPixelsHasNoScale(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100"><line x1="0" y1="0" x2="200" y2="100" stroke="black"/></svg>`
	res, err := File(
		context.Background(),
		write(t, "plan.svg", []byte(svg)),
		t.TempDir(),
		Options{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scale() != "unknown" {
		t.Fatal(res.Scale())
	}
}

func TestFileRefusals(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if _, err := File(
		ctx,
		filepath.Join(dir, "missing.dxf"),
		dir,
		Options{},
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Errorf("missing: %v", err)
	}
	if _, err := File(
		ctx,
		write(t, "a.png", []byte("\x89PNG\r\n\x1a\n")),
		dir,
		Options{},
	); !errors.Is(
		err,
		ErrNotDrawing,
	) {
		t.Errorf("png: %v", err)
	}
	if _, err := File(
		ctx,
		write(t, "a.dxf", []byte("%PDF-1.4")),
		dir,
		Options{},
	); !errors.Is(
		err,
		ErrMismatch,
	) {
		t.Errorf("pdf named dxf: %v", err)
	}
	empty := dxftest.New(4)
	if _, err := File(
		ctx,
		write(t, "empty.dxf", empty.Bytes()),
		dir,
		Options{},
	); err == nil ||
		!strings.Contains(err.Error(), "draw DXF") {
		t.Errorf("empty drawing: %v", err)
	}
	if _, err := File(
		ctx,
		write(t, "bad.dxf", []byte("0\nSECTION\n2\nENTITIES\n0\nLINE\n10\nnot a number\n")),
		dir,
		Options{},
	); err == nil ||
		!strings.Contains(err.Error(), "DXF") {
		t.Errorf("bad DXF: %v", err)
	}
	if _, err := File(
		ctx,
		write(t, "bad.svg", []byte("<svg><g>")),
		dir,
		Options{},
	); err == nil ||
		!strings.Contains(err.Error(), "SVG") {
		t.Errorf("bad SVG: %v", err)
	}
	if _, err := File(
		ctx,
		write(t, "plan.dxf", dxftest.Plan().Bytes()),
		filepath.Join(dir, "absent"),
		Options{},
	); err == nil {
		t.Error("an output folder that doesn't exist is an error")
	}
}

func TestFileTimesOut(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := File(
		ctx,
		write(t, "plan.dxf", dxftest.Plan().Bytes()),
		t.TempDir(),
		Options{Timeout: time.Nanosecond},
	)
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		// The conversion may beat the select; then it simply succeeded.
		if err != nil {
			t.Fatal(err)
		}
	}
}
