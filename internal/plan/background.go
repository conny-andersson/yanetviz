package plan

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// SVGs bundled with the application are trusted. User uploads are raster only.
func validateBackground(s string) error {
	if s == "" || oneOf(s, "/floorplans/basement.svg", "/floorplans/entrance.svg", "/floorplans/attic.svg", "/floorplans/garage.svg") {
		return nil
	}
	prefix, data, ok := strings.Cut(s, ",")
	if !ok || !oneOf(prefix, "data:image/png;base64", "data:image/jpeg;base64") {
		return errors.New("background must be a PNG or JPEG")
	}
	if len(data) > 12<<20 {
		return errors.New("background is too large (maximum 8 MiB)")
	}
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(b) > 8<<20 {
		return errors.New("invalid image data")
	}
	c, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || c.Width < 1 || c.Height < 1 || c.Width > 16000 || c.Height > 16000 || c.Width*c.Height > 64000000 {
		return errors.New("invalid image dimensions (maximum 64 megapixels)")
	}
	if !strings.Contains(prefix, format) {
		return errors.New("image type does not match its content")
	}
	return nil
}
