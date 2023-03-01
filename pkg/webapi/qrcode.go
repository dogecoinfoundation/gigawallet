package webapi

import (
	qrcode "github.com/skip2/go-qrcode"
)

func GenerateQRCodePNG(content string, size int) ([]byte, error) {
	// Generate the QR code as a PNG image
	pngBytes, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return []byte{}, err
	}
	return pngBytes, nil
}
