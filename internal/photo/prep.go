// Package photo prepares an uploaded image for the model, in memory only.
package photo

import (
	"bytes"
	"image"
	"image/jpeg"
	_ "image/png" // PNG uploads

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // WebP uploads
)

const (
	MaxSide   = 1024       // long side sent to the model
	maxPixels = 60_000_000 // larger than any phone photo
	quality   = 85
)

// A Problem is an error a person can act on.
type Problem string

func (p Problem) Error() string { return string(p) }

const (
	ErrNotPhoto Problem = "This file isn't a photo we can read. Use a JPEG, PNG or WebP picture."
	ErrTooBig   Problem = "This photo is too large. Try a smaller picture."
)

// Prepare decodes the upload, scales it so the long side is at most
// MaxSide, and re-encodes it as JPEG. Re-encoding drops EXIF and any other
// metadata. Nothing touches the disk.
func Prepare(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotPhoto
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, ErrTooBig
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotPhoto
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, fit(img, MaxSide), &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fit scales img down to side and paints it on white, so transparent
// PNGs do not turn black in the JPEG.
func fit(img image.Image, side int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > side || h > side {
		if w >= h {
			w, h = side, max(1, h*side/w)
		} else {
			w, h = max(1, w*side/h), side
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	if w == b.Dx() && h == b.Dy() {
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Over)
	} else {
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	}
	return dst
}
