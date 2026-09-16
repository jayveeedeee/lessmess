package server

// brandassets.go renders the lessmess brand assets — the SVG icon,
// the favicon ICO, and the apple-touch PNG — with the effective accent
// injected at request time, so the browser-chrome identity follows the
// ui.accent setting like the rest of the UI. The static orange files
// under web/static stay in place as fallbacks; pages link to these
// routes instead.
//
// The SVG keeps the "lm" lettering (browsers render text natively); the
// rasters are letterless accent circles/tiles — pure image/draw, no font
// dependency. The icon uses the palette's dark-theme base value as the
// single identity color: a favicon cannot observe the app's theme
// toggle, and one stable color per install is the point.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"sync"
)

// brandSVGFormat is the static icon.svg markup with the circle fill as
// the single substitution.
const brandSVGFormat = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
  <circle cx="32" cy="32" r="32" fill="%s"/>
  <text x="32" y="33" text-anchor="middle" dominant-baseline="central" font-family="'Helvetica Neue', Helvetica, Arial, sans-serif" font-weight="700" font-size="30" letter-spacing="-1" fill="#ffffff">lm</text>
</svg>
`

// brandBundle is one accent's fully rendered asset set.
type brandBundle struct {
	svg []byte
	ico []byte
	png []byte // apple-touch tile
}

// brandRenderer serves the dynamic brand routes for one repository,
// memoizing rendered assets per accent id.
type brandRenderer struct {
	resolve func() AccentColor

	mu   sync.Mutex
	memo map[string]brandBundle
}

func newBrandRenderer(resolve func() AccentColor) *brandRenderer {
	return &brandRenderer{resolve: resolve}
}

func (b *brandRenderer) current() (AccentColor, brandBundle) {
	a := b.resolve()
	b.mu.Lock()
	defer b.mu.Unlock()
	if bundle, ok := b.memo[a.ID]; ok {
		return a, bundle
	}
	bundle := renderBrandBundle(a)
	if b.memo == nil {
		b.memo = make(map[string]brandBundle)
	}
	b.memo[a.ID] = bundle
	return a, bundle
}

// serve writes one bundle part with accent-keyed revalidation: ETag +
// no-cache so a settings change propagates immediately via cheap 304s.
func (b *brandRenderer) serve(w http.ResponseWriter, r *http.Request, part func(brandBundle) []byte, contentType string) {
	a, bundle := b.current()
	etag := `"` + a.ID + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(part(bundle))
}

func (b *brandRenderer) svg(w http.ResponseWriter, r *http.Request) {
	b.serve(w, r, func(bb brandBundle) []byte { return bb.svg }, "image/svg+xml")
}

func (b *brandRenderer) ico(w http.ResponseWriter, r *http.Request) {
	b.serve(w, r, func(bb brandBundle) []byte { return bb.ico }, "image/x-icon")
}

func (b *brandRenderer) touch(w http.ResponseWriter, r *http.Request) {
	b.serve(w, r, func(bb brandBundle) []byte { return bb.png }, "image/png")
}

// renderBrandBundle renders all three assets for one accent.
func renderBrandBundle(a AccentColor) brandBundle {
	c := mustHexColor(a.Dark)
	pngs := make(map[int][]byte, 3)
	for _, size := range []int{16, 32, 48} {
		pngs[size] = encodePNG(accentCircle(size, c))
	}
	return brandBundle{
		svg: []byte(fmt.Sprintf(brandSVGFormat, a.Dark)),
		ico: buildICO(pngs),
		png: encodePNG(accentTile(180, c)),
	}
}

// mustHexColor parses a #rrggbb palette constant; the palette is
// compile-time data, so a malformed value would be a programming error.
func mustHexColor(hex string) color.RGBA {
	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		panic("brandassets: bad palette hex " + hex)
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// accentCircle draws a size×size accent circle on transparency with a
// one-pixel coverage ramp for smooth edges at favicon sizes.
func accentCircle(size int, c color.RGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	half := float64(size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - half
			dy := float64(y) + 0.5 - half
			dist := math.Sqrt(dx*dx + dy*dy)
			cov := half - dist + 0.5 // signed distance at the pixel center
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			i := img.PixOffset(x, y)
			img.Pix[i] = c.R
			img.Pix[i+1] = c.G
			img.Pix[i+2] = c.B
			img.Pix[i+3] = uint8(float64(c.A) * cov)
		}
	}
	return img
}

// accentTile draws a solid size×size accent tile (apple-touch icons are
// opaque — iOS composites transparency onto black).
func accentTile(size int, c color.RGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i] = c.R
			img.Pix[i+1] = c.G
			img.Pix[i+2] = c.B
			img.Pix[i+3] = c.A
		}
	}
	return img
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic("brandassets: png encode: " + err.Error())
	}
	return buf.Bytes()
}

// buildICO packs PNG blobs into an ICO container (PNG-compressed entries
// are a standard ICO encoding supported by every modern browser).
func buildICO(pngs map[int][]byte) []byte {
	sizes := []int{16, 32, 48}
	total := 6 + 16*len(sizes)
	for _, s := range sizes {
		total += len(pngs[s])
	}
	out := make([]byte, 0, total)
	dir := make([]byte, 6)
	binary.LittleEndian.PutUint16(dir[2:], 1) // type: icon
	binary.LittleEndian.PutUint16(dir[4:], uint16(len(sizes)))
	out = append(out, dir...)
	offset := uint32(6 + 16*len(sizes))
	for _, s := range sizes {
		png := pngs[s]
		e := make([]byte, 16)
		e[0] = uint8(s) // 48 max here; 0 would mean 256
		e[1] = uint8(s)
		binary.LittleEndian.PutUint16(e[4:], 1)  // color planes
		binary.LittleEndian.PutUint16(e[6:], 32) // bits per pixel
		binary.LittleEndian.PutUint32(e[8:], uint32(len(png)))
		binary.LittleEndian.PutUint32(e[12:], offset)
		out = append(out, e...)
		offset += uint32(len(png))
	}
	for _, s := range sizes {
		out = append(out, pngs[s]...)
	}
	return out
}
