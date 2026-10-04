package content

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestReadBoundedStopsAtConfiguredByteLimit(t *testing.T) {
	data, err := ReadBounded(strings.NewReader("12345"), 5)
	if err != nil || string(data) != "12345" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	if _, err := ReadBounded(strings.NewReader("123456"), 5); errorCategoryOf(err) != ErrorTooLarge {
		t.Fatalf("oversize error=%v category=%q", err, errorCategoryOf(err))
	}
	if _, err := ReadBounded(strings.NewReader("x"), 0); errorCategoryOf(err) != ErrorValidation {
		t.Fatalf("invalid limit error=%v category=%q", err, errorCategoryOf(err))
	}
}

func TestImageBudgetRejectsInvalidOrLooserOverrides(t *testing.T) {
	hard := HardImageBudget()
	invalid := []ImageBudget{
		{},
		{MaxDimension: hard.MaxDimension + 1, MaxPixels: hard.MaxPixels, MaxDecodedBytes: hard.MaxDecodedBytes, MaxGIFFrames: hard.MaxGIFFrames},
		{MaxDimension: hard.MaxDimension, MaxPixels: hard.MaxPixels + 1, MaxDecodedBytes: hard.MaxDecodedBytes, MaxGIFFrames: hard.MaxGIFFrames},
		{MaxDimension: hard.MaxDimension, MaxPixels: hard.MaxPixels, MaxDecodedBytes: hard.MaxDecodedBytes + 1, MaxGIFFrames: hard.MaxGIFFrames},
		{MaxDimension: hard.MaxDimension, MaxPixels: hard.MaxPixels, MaxDecodedBytes: hard.MaxDecodedBytes, MaxGIFFrames: hard.MaxGIFFrames + 1},
	}
	for _, budget := range invalid {
		if _, err := ResolveImageBudget(&budget); errorCategoryOf(err) != ErrorValidation {
			t.Fatalf("budget=%+v err=%v", budget, err)
		}
	}
	tight := ImageBudget{MaxDimension: 64, MaxPixels: 4096, MaxDecodedBytes: 16384, MaxGIFFrames: 2}
	if got, err := ResolveImageBudget(&tight); err != nil || got != tight {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestValidateImageAcceptsRealBaselineFormats(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		data      []byte
		format    string
	}{
		{name: "png", extension: ".png", data: encodePNG(t, 2, 3), format: "png"},
		{name: "jpeg", extension: ".jpg", data: encodeJPEG(t, 2, 3), format: "jpeg"},
		{name: "gif", extension: ".gif", data: encodeGIF(t, 2, 3, 2), format: "gif"},
		{name: "bmp", extension: ".bmp", data: onePixelBMP(), format: "bmp"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata, err := ValidateImage(test.data, test.extension, HardImageBudget())
			if err != nil || metadata.Format != test.format || metadata.Width < 1 || metadata.Height < 1 {
				t.Fatalf("metadata=%+v err=%v", metadata, err)
			}
		})
	}
}

func TestValidateImageRejectsExtensionMismatchAndTruncation(t *testing.T) {
	if _, err := ValidateImage(encodePNG(t, 1, 1), ".jpg", HardImageBudget()); errorCategoryOf(err) != ErrorUnsupportedMedia {
		t.Fatalf("mismatch error=%v category=%q", err, errorCategoryOf(err))
	}
	pngData := encodePNG(t, 1, 1)
	if _, err := ValidateImage(pngData[:len(pngData)-4], ".png", HardImageBudget()); errorCategoryOf(err) != ErrorUnsupportedMedia {
		t.Fatalf("truncated error=%v category=%q", err, errorCategoryOf(err))
	}
}

func TestValidateImageDerivesFormatWhenTransportHasNoFilenameExtension(t *testing.T) {
	metadata, err := ValidateImage(encodeJPEG(t, 1, 1), "", HardImageBudget())
	if err != nil || metadata.Format != "jpeg" || metadata.Extension != ".jpg" {
		t.Fatalf("metadata=%+v err=%v", metadata, err)
	}
}

func TestValidateImageRejectsOversizedHeaderBeforeFullDecode(t *testing.T) {
	data := oversizedPNGHeader(20_000, 2)
	if _, err := ValidateImage(data, ".png", HardImageBudget()); errorCategoryOf(err) != ErrorTooLarge {
		t.Fatalf("oversized header error=%v category=%q", err, errorCategoryOf(err))
	}
}

func TestValidateImageRejectsGIFFrameAndCumulativePixelBudgets(t *testing.T) {
	threeFrames := encodeGIF(t, 2, 2, 3)
	frameBudget := ImageBudget{MaxDimension: 10, MaxPixels: 100, MaxDecodedBytes: 400, MaxGIFFrames: 2}
	if _, err := ValidateImage(threeFrames, ".gif", frameBudget); errorCategoryOf(err) != ErrorTooLarge {
		t.Fatalf("frame error=%v category=%q", err, errorCategoryOf(err))
	}
	twoFrames := encodeGIF(t, 3, 3, 2)
	pixelBudget := ImageBudget{MaxDimension: 10, MaxPixels: 12, MaxDecodedBytes: 100, MaxGIFFrames: 3}
	if _, err := ValidateImage(twoFrames, ".gif", pixelBudget); errorCategoryOf(err) != ErrorTooLarge {
		t.Fatalf("pixel error=%v category=%q", err, errorCategoryOf(err))
	}
}

func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func encodeJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func encodeGIF(t *testing.T, width, height, frames int) []byte {
	t.Helper()
	animation := &gif.GIF{}
	palette := color.Palette{color.Black, color.White}
	for index := 0; index < frames; index++ {
		animation.Image = append(animation.Image, image.NewPaletted(image.Rect(0, 0, width, height), palette))
		animation.Delay = append(animation.Delay, 0)
	}
	var buffer bytes.Buffer
	if err := gif.EncodeAll(&buffer, animation); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func oversizedPNGHeader(width, height uint32) []byte {
	data := append([]byte(nil), []byte("\x89PNG\r\n\x1a\n")...)
	payload := make([]byte, 13)
	binary.BigEndian.PutUint32(payload[0:4], width)
	binary.BigEndian.PutUint32(payload[4:8], height)
	payload[8], payload[9], payload[10], payload[11], payload[12] = 8, 6, 0, 0, 0
	chunkType := []byte("IHDR")
	chunk := make([]byte, 8+len(payload)+4)
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(payload)))
	copy(chunk[4:8], chunkType)
	copy(chunk[8:], payload)
	binary.BigEndian.PutUint32(chunk[8+len(payload):], crc32.ChecksumIEEE(append(chunkType, payload...)))
	return append(data, chunk...)
}

func onePixelBMP() []byte {
	return []byte{
		'B', 'M', 58, 0, 0, 0, 0, 0, 0, 0, 54, 0, 0, 0,
		40, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 24, 0,
		0, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 0,
	}
}

func errorCategoryOf(err error) ErrorCategory {
	var contentErr *Error
	if errors.As(err, &contentErr) {
		return contentErr.Category
	}
	return ""
}
