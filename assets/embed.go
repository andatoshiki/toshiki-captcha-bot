package assets

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
)

// Embedded captcha assets to avoid runtime filesystem dependency.
//
//go:embed image/gopherbg.jpg image/emoji/*.png
var files embed.FS

func LoadBackground() (image.Image, error) {
	return decodeJPEG("image/gopherbg.jpg")
}

func LoadEmojiByKey(key string) (image.Image, error) {
	clean := strings.TrimSpace(key)
	if clean == "" {
		return nil, fmt.Errorf("emoji key must not be empty")
	}
	path := fmt.Sprintf("image/emoji/%s.png", clean)
	return decodePNG(path)
}

func decodeJPEG(path string) (image.Image, error) {
	data, err := files.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read embedded asset %q: %w", path, err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode embedded jpeg %q: %w", path, err)
	}
	return img, nil
}

func decodePNG(path string) (image.Image, error) {
	data, err := files.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read embedded asset %q: %w", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode embedded png %q: %w", path, err)
	}
	return img, nil
}
