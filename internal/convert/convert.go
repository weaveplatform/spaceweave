package convert

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dwg"
	"github.com/weaveplatform/spaceweave/internal/dxf"
	"github.com/weaveplatform/spaceweave/internal/svgdraw"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// The files a conversion writes into its output folder.
const (
	ImageFile   = "source.png"
	DrawingFile = "drawing.svg"
)

// DefaultTimeout bounds one conversion. Readers don't watch the context, so
// a timed-out conversion finishes in the background and is thrown away; the
// item and nesting caps in cad and svgdraw keep that bounded.
const DefaultTimeout = 2 * time.Minute

// ErrNotDrawing is returned for a raster image, which needs no conversion.
var ErrNotDrawing = errors.New("this is already an image; only drawings are converted")

// Options control a conversion.
type Options struct {
	MaxSide   int // the image's long edge in pixels; vector.DefaultMaxSide when 0
	MaxPixels int // a cap on width × height; none when 0
	Timeout   time.Duration
}

// Result is what a conversion produced.
type Result struct {
	Format Format
	Image  string // path of the PNG
	SVG    string // path of the SVG
	Width  int
	Height int
	// PxPerMetre is the image's scale on the building, when the drawing's
	// units say it; 0 otherwise. Assumed means the units were guessed from
	// the drawing's size.
	PxPerMetre float64
	Assumed    bool
	// PxPerPaperMM is the image's scale on paper, for a drawing (SVG in mm,
	// say) whose units are a sheet's rather than the building's; 0 otherwise.
	PxPerPaperMM float64
	// Trimmed is how many stray items far from the rest were left out.
	Trimmed int
	// View maps the drawing's coordinates to the image's pixels, so a
	// tracer reading the drawing itself puts things where the image shows
	// them.
	View  vector.Affine
	Notes []string
}

// Scale describes the result's scale for people and for the tracer.
func (r *Result) Scale() string {
	switch {
	case r.PxPerMetre > 0 && r.Assumed:
		return fmt.Sprintf("%.2f px per metre (assumed from the drawing's size)", r.PxPerMetre)
	case r.PxPerMetre > 0:
		return fmt.Sprintf("%.2f px per metre (from the drawing's units)", r.PxPerMetre)
	case r.PxPerPaperMM > 0:
		return fmt.Sprintf(
			"%.3f px per mm on paper; the drawing's scale on paper isn't known",
			r.PxPerPaperMM,
		)
	default:
		return "unknown"
	}
}

// File converts the drawing at path into dir, as ImageFile and DrawingFile.
func File(ctx context.Context, path, dir string, o Options) (*Result, error) {
	f, err := os.Open(path) //nolint:gosec // the caller's own copy of an upload
	if err != nil {
		return nil, fmt.Errorf("open drawing: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only
	head := make([]byte, SniffLen)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read drawing: %w", err)
	}
	format, err := Check(filepath.Base(path), head[:n])
	if err != nil {
		return nil, err
	}
	if !format.Drawing {
		return nil, ErrNotDrawing
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("read drawing: %w", err)
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type out struct {
		r   *Result
		err error
	}
	done := make(chan out, 1)
	go func() {
		r, err := convert(f, format, dir, o)
		done <- out{r, err}
	}()
	select {
	case res := <-done:
		return res.r, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("convert drawing: %w", ctx.Err())
	}
}

func read(r io.Reader, format Format) (*vector.Drawing, error) {
	var readCAD func(io.Reader) (*cad.Document, error)
	switch format.ID {
	case DXF.ID:
		readCAD = dxf.Read
	case DWG.ID:
		readCAD = dwg.Read
	}
	if readCAD != nil {
		// Both readers produce the same document, so a DWG gets the DXF's
		// units, scale and view: cad.Draw is the one place they're decided.
		doc, err := readCAD(r)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", format.Label, err)
		}
		d, err := cad.Draw(doc)
		if err != nil {
			return nil, fmt.Errorf("draw %s: %w", format.Label, err)
		}
		return d, nil
	}
	// SVG; Check has ruled out everything else.
	d, err := svgdraw.Read(r)
	if err != nil {
		return nil, fmt.Errorf("read SVG: %w", err)
	}
	return d, nil
}

func convert(r io.Reader, format Format, dir string, o Options) (*Result, error) {
	d, err := read(r, format)
	if err != nil {
		return nil, err
	}
	ras, err := vector.Rasterize(d, vector.Options{MaxSide: o.MaxSide, MaxPixels: o.MaxPixels})
	if err != nil {
		return nil, fmt.Errorf("draw %s: %w", format.Label, err)
	}
	res := &Result{
		Format:  format,
		Image:   filepath.Join(dir, ImageFile),
		SVG:     filepath.Join(dir, DrawingFile),
		Width:   ras.Image.Bounds().Dx(),
		Height:  ras.Image.Bounds().Dy(),
		Trimmed: ras.Trimmed,
		Notes:   d.Notes,
		View:    ras.View(),
	}
	if d.Unit > 0 {
		perMetre := ras.PxPerUnit / d.Unit
		if d.Paper {
			res.PxPerPaperMM = perMetre / 1000
		} else {
			res.PxPerMetre, res.Assumed = perMetre, d.Assumed
		}
	}
	switch {
	case ras.Trimmed == 1:
		res.Notes = append(
			res.Notes,
			"1 stray item far from the rest of the drawing was left out of the picture.",
		)
	case ras.Trimmed > 1:
		res.Notes = append(
			res.Notes,
			fmt.Sprintf(
				"%d stray items far from the rest of the drawing were left out of the picture.",
				ras.Trimmed,
			),
		)
	}
	var img bytes.Buffer
	if err := png.Encode(&img, ras.Image); err != nil {
		return nil, fmt.Errorf("encode image: %w", err)
	}
	if err := os.WriteFile(res.Image, img.Bytes(), 0o600); err != nil {
		return nil, fmt.Errorf("write image: %w", err)
	}
	var svg bytes.Buffer
	if err := vector.WriteSVG(&svg, d, ras.Bounds); err != nil {
		return nil, fmt.Errorf("write SVG: %w", err)
	}
	if err := os.WriteFile(res.SVG, svg.Bytes(), 0o600); err != nil {
		return nil, fmt.Errorf("write SVG: %w", err)
	}
	return res, nil
}
