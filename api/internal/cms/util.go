package cms

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func MustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func ValidTheme(t string) bool {
	switch t {
	case ThemeBAContent, ThemePanoramaGallery, ThemeTextContent:
		return true
	default:
		return false
	}
}

func ValidBlockType(t string) bool {
	switch t {
	case BlockComparisonSlider, BlockGalleryImage, BlockRichText, BlockContactForm:
		return true
	default:
		return false
	}
}

// DefaultAllowedBlocks returns the block types that fit a theme engine.
func DefaultAllowedBlocks(theme string) []string {
	switch theme {
	case ThemeBAContent:
		return []string{BlockComparisonSlider}
	case ThemePanoramaGallery:
		return []string{BlockGalleryImage}
	case ThemeTextContent:
		return []string{BlockRichText, BlockContactForm}
	default:
		return []string{}
	}
}
