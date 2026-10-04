package content

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strings"

	_ "golang.org/x/image/bmp"
)

const (
	hardMaxImageDimension    = 16_384
	hardMaxImagePixels       = 25_000_000
	hardMaxImageDecodedBytes = 128 * 1024 * 1024
	hardMaxGIFFrames         = 256
	decodedBytesPerPixel     = 4
)

type ImageBudget struct {
	MaxDimension    int
	MaxPixels       uint64
	MaxDecodedBytes uint64
	MaxGIFFrames    int
}

type ImageMetadata struct {
	Format    string
	MIME      string
	Extension string
	Width     int
	Height    int
	Frames    int
}

func HardImageBudget() ImageBudget {
	return ImageBudget{
		MaxDimension:    hardMaxImageDimension,
		MaxPixels:       hardMaxImagePixels,
		MaxDecodedBytes: hardMaxImageDecodedBytes,
		MaxGIFFrames:    hardMaxGIFFrames,
	}
}

// ResolveImageBudget applies an optional deployment override.  Overrides may
// only tighten the built-in profile; malformed, zero, or looser values fail
// closed instead of silently restoring a permissive default.
func ResolveImageBudget(override *ImageBudget) (ImageBudget, error) {
	hard := HardImageBudget()
	if override == nil {
		return hard, nil
	}
	if override.MaxDimension <= 0 || override.MaxPixels == 0 || override.MaxDecodedBytes == 0 || override.MaxGIFFrames <= 0 {
		return ImageBudget{}, validationError("invalid_image_budget", nil)
	}
	if override.MaxDimension > hard.MaxDimension || override.MaxPixels > hard.MaxPixels ||
		override.MaxDecodedBytes > hard.MaxDecodedBytes || override.MaxGIFFrames > hard.MaxGIFFrames {
		return ImageBudget{}, validationError("image_budget_exceeds_hard_limit", nil)
	}
	return *override, nil
}

// ReadBounded consumes at most limit+1 bytes so an oversized input is detected
// without trusting Content-Length or reading the complete body.
func ReadBounded(reader io.Reader, limit int64) ([]byte, error) {
	if reader == nil || limit <= 0 || limit == math.MaxInt64 {
		return nil, validationError("invalid_stream_limit", nil)
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, contentError(ErrorStorageUnavailable, "read_input", err)
	}
	if int64(len(data)) > limit {
		return nil, contentError(ErrorTooLarge, "compressed_size_limit", nil)
	}
	return data, nil
}

func ValidateImage(data []byte, extension string, budget ImageBudget) (ImageMetadata, error) {
	budget, err := ResolveImageBudget(&budget)
	if err != nil {
		return ImageMetadata{}, err
	}
	if len(data) == 0 {
		return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "empty_image", nil)
	}
	extension = strings.ToLower(strings.TrimSpace(extension))
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "decode_image_header", err)
	}
	if err := validateImageDimensions(config.Width, config.Height, 1, budget); err != nil {
		return ImageMetadata{}, err
	}
	mime, canonicalExtension, expectedExtension, ok := imageFormatContract(format)
	if !ok || (extension != "" && !expectedExtension(extension)) {
		return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "image_extension_mismatch", nil)
	}
	if detected := http.DetectContentType(data); detected != mime {
		return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "image_sniff_mismatch", nil)
	}

	metadata := ImageMetadata{Format: format, MIME: mime, Extension: canonicalExtension, Width: config.Width, Height: config.Height, Frames: 1}
	if format == "gif" {
		frames, err := inspectGIFFrames(data, budget)
		if err != nil {
			return ImageMetadata{}, err
		}
		animation, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "decode_gif", err)
		}
		if len(animation.Image) != frames {
			return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "gif_frame_count_changed", nil)
		}
		metadata.Frames = frames
		return metadata, nil
	}

	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "decode_image", err)
	}
	if decodedFormat != format {
		return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "decoder_format_changed", nil)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != config.Width || bounds.Dy() != config.Height {
		return ImageMetadata{}, contentError(ErrorUnsupportedMedia, "decoded_dimensions_changed", nil)
	}
	return metadata, nil
}

func validateImageDimensions(width, height int, frames uint64, budget ImageBudget) error {
	if width <= 0 || height <= 0 {
		return contentError(ErrorUnsupportedMedia, "invalid_image_dimensions", nil)
	}
	if width > budget.MaxDimension || height > budget.MaxDimension {
		return contentError(ErrorTooLarge, "image_dimension_limit", nil)
	}
	pixels, ok := checkedMultiply(uint64(width), uint64(height))
	if !ok {
		return contentError(ErrorTooLarge, "image_pixel_overflow", nil)
	}
	pixels, ok = checkedMultiply(pixels, frames)
	if !ok || pixels > budget.MaxPixels {
		return contentError(ErrorTooLarge, "image_pixel_limit", nil)
	}
	decodedBytes, ok := checkedMultiply(pixels, decodedBytesPerPixel)
	if !ok || decodedBytes > budget.MaxDecodedBytes {
		return contentError(ErrorTooLarge, "image_allocation_limit", nil)
	}
	return nil
}

func checkedMultiply(left, right uint64) (uint64, bool) {
	if left != 0 && right > math.MaxUint64/left {
		return 0, false
	}
	return left * right, true
}

func imageFormatContract(format string) (string, string, func(string) bool, bool) {
	switch format {
	case "png":
		return "image/png", ".png", func(extension string) bool { return extension == ".png" }, true
	case "jpeg":
		return "image/jpeg", ".jpg", func(extension string) bool { return extension == ".jpg" || extension == ".jpeg" }, true
	case "gif":
		return "image/gif", ".gif", func(extension string) bool { return extension == ".gif" }, true
	case "bmp":
		return "image/bmp", ".bmp", func(extension string) bool { return extension == ".bmp" }, true
	default:
		return "", "", nil, false
	}
}

// inspectGIFFrames walks GIF blocks without decompressing their image data.
// This enforces frame and cumulative allocation limits before gif.DecodeAll
// can allocate every frame.
func inspectGIFFrames(data []byte, budget ImageBudget) (int, error) {
	if len(data) < 13 || (string(data[:6]) != "GIF87a" && string(data[:6]) != "GIF89a") {
		return 0, contentError(ErrorUnsupportedMedia, "invalid_gif_header", nil)
	}
	position := 13
	packed := data[10]
	if packed&0x80 != 0 {
		position += 3 * (1 << ((packed & 0x07) + 1))
	}
	if position > len(data) {
		return 0, contentError(ErrorUnsupportedMedia, "truncated_gif_color_table", io.ErrUnexpectedEOF)
	}
	frames := 0
	var cumulativePixels uint64
	for position < len(data) {
		block := data[position]
		position++
		switch block {
		case 0x3b:
			if frames == 0 {
				return 0, contentError(ErrorUnsupportedMedia, "gif_without_frames", nil)
			}
			return frames, nil
		case 0x21:
			if position >= len(data) {
				return 0, contentError(ErrorUnsupportedMedia, "truncated_gif_extension", io.ErrUnexpectedEOF)
			}
			position++ // extension label
			var err error
			position, err = skipGIFSubBlocks(data, position)
			if err != nil {
				return 0, err
			}
		case 0x2c:
			if position+9 > len(data) {
				return 0, contentError(ErrorUnsupportedMedia, "truncated_gif_descriptor", io.ErrUnexpectedEOF)
			}
			width := int(binary.LittleEndian.Uint16(data[position+4 : position+6]))
			height := int(binary.LittleEndian.Uint16(data[position+6 : position+8]))
			framePacked := data[position+8]
			position += 9
			if framePacked&0x80 != 0 {
				position += 3 * (1 << ((framePacked & 0x07) + 1))
			}
			if position >= len(data) {
				return 0, contentError(ErrorUnsupportedMedia, "truncated_gif_image", io.ErrUnexpectedEOF)
			}
			position++ // LZW minimum code size
			var err error
			position, err = skipGIFSubBlocks(data, position)
			if err != nil {
				return 0, err
			}
			frames++
			if frames > budget.MaxGIFFrames {
				return 0, contentError(ErrorTooLarge, "gif_frame_limit", nil)
			}
			pixels, ok := checkedMultiply(uint64(width), uint64(height))
			if !ok || cumulativePixels > math.MaxUint64-pixels {
				return 0, contentError(ErrorTooLarge, "gif_pixel_overflow", nil)
			}
			cumulativePixels += pixels
			if cumulativePixels > budget.MaxPixels {
				return 0, contentError(ErrorTooLarge, "gif_pixel_limit", nil)
			}
			decodedBytes, ok := checkedMultiply(cumulativePixels, decodedBytesPerPixel)
			if !ok || decodedBytes > budget.MaxDecodedBytes {
				return 0, contentError(ErrorTooLarge, "gif_allocation_limit", nil)
			}
		default:
			return 0, contentError(ErrorUnsupportedMedia, "unknown_gif_block", fmt.Errorf("block 0x%02x", block))
		}
	}
	return 0, contentError(ErrorUnsupportedMedia, "missing_gif_trailer", errors.New("unexpected EOF"))
}

func skipGIFSubBlocks(data []byte, position int) (int, error) {
	for position < len(data) {
		size := int(data[position])
		position++
		if size == 0 {
			return position, nil
		}
		if position+size > len(data) {
			return 0, contentError(ErrorUnsupportedMedia, "truncated_gif_sub_block", io.ErrUnexpectedEOF)
		}
		position += size
	}
	return 0, contentError(ErrorUnsupportedMedia, "truncated_gif_sub_block", io.ErrUnexpectedEOF)
}
