package server

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrandBundleShape(t *testing.T) {
	a := DefaultAccent()
	bb := renderBrandBundle(a)

	svg := string(bb.svg)
	for _, want := range []string{`fill="` + a.Dark + `"`, ">lm<", `viewBox="0 0 64 64"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %q", want)
		}
	}

	// ICO: magic, type, three entries whose blobs decode as PNGs of the
	// right sizes at the declared offsets.
	if len(bb.ico) < 6 || !bytes.Equal(bb.ico[:4], []byte{0, 0, 1, 0}) {
		t.Fatalf("ico magic = %v", bb.ico[:min(4, len(bb.ico))])
	}
	if n := binary.LittleEndian.Uint16(bb.ico[4:6]); n != 3 {
		t.Fatalf("ico image count = %d, want 3", n)
	}
	wantSizes := []uint8{16, 32, 48}
	for i, ws := range wantSizes {
		e := bb.ico[6+16*i : 6+16*i+16]
		if e[0] != ws || e[1] != ws {
			t.Errorf("entry %d size = %d/%d, want %d", i, e[0], e[1], ws)
		}
		sz := binary.LittleEndian.Uint32(e[8:12])
		off := binary.LittleEndian.Uint32(e[12:16])
		if int(off+sz) > len(bb.ico) {
			t.Fatalf("entry %d blob out of range: off %d size %d", i, off, sz)
		}
		img, err := png.Decode(bytes.NewReader(bb.ico[off : off+sz]))
		if err != nil {
			t.Fatalf("entry %d png: %v", i, err)
		}
		if img.Bounds().Dx() != int(ws) || img.Bounds().Dy() != int(ws) {
			t.Errorf("entry %d decoded %dx%d, want %d", i, img.Bounds().Dx(), img.Bounds().Dy(), ws)
		}
	}

	// Apple-touch tile: opaque PNG at 180×180.
	tile, err := png.Decode(bytes.NewReader(bb.png))
	if err != nil {
		t.Fatalf("tile png: %v", err)
	}
	if tile.Bounds().Dx() != 180 || tile.Bounds().Dy() != 180 {
		t.Errorf("tile = %v, want 180x180", tile.Bounds())
	}
	if op, ok := tile.(interface{ Opaque() bool }); ok && !op.Opaque() {
		t.Error("apple-touch tile must be opaque (iOS composites transparency onto black)")
	}
}

func TestAccentCircleAndTile(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	img := accentCircle(32, red)
	if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
		t.Fatalf("bounds = %v", img.Bounds())
	}
	center := img.NRGBAAt(16, 16)
	if center.A != 255 || center.R != 255 {
		t.Errorf("center = %v, want opaque red", center)
	}
	if corner := img.NRGBAAt(0, 0); corner.A != 0 {
		t.Errorf("corner = %v, want transparent", corner)
	}
	// Anti-aliasing: the ramp must produce partially covered pixels.
	var partial int
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if a := img.NRGBAAt(x, y).A; a > 0 && a < 255 {
				partial++
			}
		}
	}
	if partial == 0 {
		t.Error("no partially covered edge pixels; circle edges are jagged")
	}

	tile := accentTile(8, red)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if got := tile.NRGBAAt(x, y); got != (color.NRGBA{R: 255, A: 255}) {
				t.Fatalf("tile pixel (%d,%d) = %v", x, y, got)
			}
		}
	}
}

func TestBrandRoutesFollowAccent(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()

	// A fixture store has no ui.accent: the first brand request rolls and
	// persists one, and the response must carry that accent's color.
	a := ResolveAccent(st.Dir)

	w := do(t, h, "GET", "/icon.svg", "")
	if w.Code != 200 {
		t.Fatalf("icon.svg code = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("icon.svg content type = %q", ct)
	}
	if !strings.Contains(w.Body.String(), `fill="`+a.Dark+`"`) {
		t.Errorf("icon.svg not rendered in rolled accent %s: %s", a.Dark, w.Body.String())
	}
	etag := w.Header().Get("ETag")
	if etag != `"`+a.ID+`"` {
		t.Errorf("etag = %q, want %q", etag, `"`+a.ID+`"`)
	}

	// Revalidation: matching If-None-Match gets a cheap 304.
	req := httptest.NewRequest("GET", "/icon.svg", nil)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("304 flow code = %d, want %d", rec.Code, http.StatusNotModified)
	}

	w = do(t, h, "GET", "/favicon.ico", "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/x-icon" {
		t.Errorf("favicon.ico = %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	if !bytes.Equal(w.Body.Bytes()[:4], []byte{0, 0, 1, 0}) {
		t.Error("favicon.ico lacks the ICO magic")
	}

	w = do(t, h, "GET", "/apple-touch-icon.png", "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("apple-touch-icon.png = %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(w.Body.Bytes())); err != nil || cfg.Width != 180 {
		t.Errorf("apple-touch tile decode = %v, %v", cfg, err)
	}
}

func TestBrandRoutesUseConfiguredAccent(t *testing.T) {
	st, dir := fixtureStore(t)
	writeJSONFile(t, settingsPersonalPath(dir), Settings{UI: UISettings{Accent: "violet"}})
	h := New(st).Handler()
	w := do(t, h, "GET", "/icon.svg", "")
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	a, _ := AccentByID("violet")
	if !strings.Contains(w.Body.String(), `fill="`+a.Dark+`"`) {
		t.Errorf("icon.svg not in configured accent: %s", w.Body.String())
	}
}

func TestLayoutCarriesAccentStyleAndDynamicIcons(t *testing.T) {
	st, dir := fixtureStore(t)
	writeJSONFile(t, settingsProjectPath(dir), Settings{UI: UISettings{Accent: "teal"}})
	w := htmlGet(t, New(st).Handler(), "/", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	a, _ := AccentByID("teal")
	for _, want := range []string{
		`:root{--accent:` + a.Dark + `;--accent-hover:` + a.DarkHover + `}`,
		`[data-theme="light"]{--accent:` + a.Light + `;--accent-hover:` + a.LightHover + `}`,
		`<link rel="icon" href="/icon.svg"`,
		`<link rel="alternate icon" href="/favicon.ico">`,
		`<link rel="apple-touch-icon" href="/apple-touch-icon.png">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("layout missing %q", want)
		}
	}
	if strings.Contains(body, `/static/icon.svg`) {
		t.Error("layout still links the static icon; pages must use the dynamic route")
	}
}
