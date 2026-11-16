package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/catalog"
	"github.com/weaveplatform/spaceweave/internal/convert"
	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/server"
)

// serving is `spaceweave serve` running on a site, stopped (by interrupt,
// so the binary writes its coverage) when the test ends.
type serving struct {
	s    *site
	base string
	log  *lockedBuffer
}

func serve(t *testing.T, s *site) *serving {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stopping the server needs an interrupt signal")
	}
	log := &lockedBuffer{}
	cmd := s.command("serve", "--addr", "127.0.0.1:0", "--site", filepath.Join(repoRoot, "web", "floorplan-iso"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-exited:
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
			t.Error("serve didn't stop on interrupt")
		}
	})
	for deadline := time.Now().Add(15 * time.Second); ; {
		if _, rest, ok := strings.Cut(log.String(), "msg=serving url="); ok {
			return &serving{s: s, base: strings.TrimSuffix(strings.Fields(rest)[0], "/"), log: log}
		}
		if time.Now().After(deadline) {
			t.Fatalf("serve didn't start:\n%s", log)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// upload sends a file the way the page does.
func (v *serving) upload(t *testing.T, name string, data []byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("image", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, v.base+"/api/plans", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(server.RequestHeader, "1")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close() //nolint:errcheck // test
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

// uploadDWG uploads a DWG to a fresh key-less site and returns its draft's
// entry and plan.json.
func uploadDWG(t *testing.T, name string, data []byte) (catalog.Entry, []byte) {
	t.Helper()
	v := serve(t, newAutoSite(t))
	if code, body := v.upload(t, name, data); code != http.StatusCreated {
		t.Fatalf("upload: %d %s", code, body)
	}
	ps := v.s.waitFor("the DWG's draft", func(ps []catalog.Entry) bool {
		return len(ps) == 1 && (ps[0].Status == catalog.Draft || ps[0].Status == catalog.Failed)
	}, v.log)
	e := ps[0]
	if e.Status != catalog.Draft || e.Tracer != "auto" || e.CostUSD != 0 || e.Format != "dwg" ||
		e.Upload != "original.dwg" || e.Source != convert.ImageFile {
		t.Fatalf("%+v\n%s", e, v.log)
	}
	planJSON, err := os.ReadFile(filepath.Join(v.s.output, e.ID, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	return e, planJSON
}

// TestDWGUploadNeedsNoKey uploads the office drawing as DWG, in the newest
// and oldest formats the reader takes, to a site with no API key, and
// checks the draft has the drawing's exact scale and its named rooms, and
// that a second site traces it to the same bytes.
func TestDWGUploadNeedsNoKey(t *testing.T) {
	for _, version := range []string{"AC1032", "AC1015"} {
		t.Run(version, func(t *testing.T) {
			data := dwgtest.Office(version)

			// The scale the drawing's units give, rendered as the worker
			// renders it.
			src := filepath.Join(t.TempDir(), "office.dwg")
			if err := os.WriteFile(src, data, 0o600); err != nil {
				t.Fatal(err)
			}
			want, err := convert.File(context.Background(), src, t.TempDir(), convert.Options{})
			if err != nil {
				t.Fatal(err)
			}

			entry, planJSON := uploadDWG(t, "office.dwg", data)
			var p struct {
				Source struct {
					PxPerMetre float64 `json:"pxPerMetre"`
				} `json:"SOURCE"`
				Rooms []struct {
					Name string `json:"name"`
				} `json:"rooms"`
			}
			if err := json.Unmarshal(planJSON, &p); err != nil {
				t.Fatal(err)
			}
			if !entry.Scale.Exact() || entry.Scale.PxPerMetre != want.PxPerMetre ||
				p.Source.PxPerMetre != roundTo(want.PxPerMetre) {
				t.Errorf("scale: plan %v, entry %+v, drawing %v",
					p.Source.PxPerMetre, entry.Scale, want.PxPerMetre)
			}
			names := make([]string, 0, len(p.Rooms))
			for _, r := range p.Rooms {
				names = append(names, r.Name)
			}
			for name := range dxftest.OfficeRooms {
				if !slices.Contains(names, name) {
					t.Errorf("room %q missing from %v", name, names)
				}
			}

			if _, again := uploadDWG(t, "office.dwg", data); !bytes.Equal(planJSON, again) {
				t.Error("the same DWG traced twice must give the same plan.json")
			}
		})
	}
}

// A DWG in a format the reader doesn't take is refused at the upload, with
// the release it is and what to save it as.
func TestDWGUploadRefusesR2007(t *testing.T) {
	v := serve(t, newAutoSite(t))
	code, body := v.upload(t, "old.dwg", []byte("AC1021\x00\x00\x00\x00\x00\x06\x01"))
	if code != http.StatusUnsupportedMediaType ||
		!strings.Contains(body, "This DWG is AutoCAD 2007 format, which isn't supported. Save it as DWG 2018 or 2013, or as DXF.") {
		t.Fatalf("%d %s", code, body)
	}
}
