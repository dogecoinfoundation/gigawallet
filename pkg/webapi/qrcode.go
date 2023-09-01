package webapi

import (
	"encoding/hex"
	"image/color"

	qrcode "github.com/skip2/go-qrcode"
)

func GenerateQRCodePNG(content string, size int, fg string, bg string) ([]byte, error) {
	q, err := qrcode.New(content, qrcode.Medium)
	q.ForegroundColor = color.White

	DefaultFG := []byte{0, 0, 0, 254}
	DefaultBG := []byte{255, 255, 255, 255}

	// decode passed in fg/bg if sent
	f, err := hex.DecodeString(fg)
	if err == nil && len(f) >= 3 {
		DefaultFG = f
	}
	b, err := hex.DecodeString(bg)
	if err == nil && len(b) >= 3 {
		DefaultBG = b
	}

	// add default alpha
	if len(DefaultFG) == 3 {
		DefaultFG = append(DefaultFG, 255)
	}

	if len(DefaultBG) == 3 {
		DefaultBG = append(DefaultBG, 255)
	}

	q.BackgroundColor = color.RGBA{DefaultBG[0], DefaultBG[1], DefaultBG[2], DefaultBG[3]}
	q.ForegroundColor = color.RGBA{DefaultFG[0], DefaultFG[1], DefaultFG[2], DefaultFG[3]}

	pngBytes, err := q.PNG(size)
	if err != nil {
		return []byte{}, err
	}
	return pngBytes, nil
}
