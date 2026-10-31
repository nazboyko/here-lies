package photo

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func pngOf(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.NRGBA{200, 10, 10, 255})
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestPrepareScalesAndReencodes(t *testing.T) {
	out, err := Prepare(pngOf(3000, 1500))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output is not JPEG: %v", err)
	}
	if cfg.Width != MaxSide || cfg.Height != 512 {
		t.Errorf("size %dx%d, want %dx512", cfg.Width, cfg.Height, MaxSide)
	}
}

func TestPrepareKeepsSmallPhotos(t *testing.T) {
	out, err := Prepare(pngOf(300, 200))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := jpeg.DecodeConfig(bytes.NewReader(out))
	if cfg.Width != 300 || cfg.Height != 200 {
		t.Errorf("size %dx%d, want 300x200", cfg.Width, cfg.Height)
	}
}

func TestPrepareRejectsNonPhotos(t *testing.T) {
	_, err := Prepare([]byte("%PDF-1.7 not a photo"))
	if !errors.Is(err, ErrNotPhoto) {
		t.Errorf("err = %v, want ErrNotPhoto", err)
	}
}
