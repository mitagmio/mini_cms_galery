package generate

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"strings"

	"sheyanova.art/api/internal/cms"
)

func newShuffleSeed() int64 {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return 1
	}
	seed := int64(binary.BigEndian.Uint64(b[:]) & 0x7fffffffffffffff)
	if seed == 0 {
		return 1
	}
	return seed
}

func (g *Generator) ensureLookbookShuffleSeed(p *cms.Page) (int64, error) {
	seed := cms.ShuffleSeed(p.Settings)
	if seed != 0 {
		return seed, nil
	}
	seed = newShuffleSeed()
	if err := g.Store.MergePageSettings(p.ID, map[string]any{"shuffle_seed": seed}); err != nil {
		return 0, err
	}
	p.Settings = cms.MergeSettings(p.Settings, map[string]any{"shuffle_seed": seed})
	return seed, nil
}

func galleryDataHasMedia(data map[string]any) bool {
	if data == nil {
		return false
	}
	mid, _ := data["media_id"].(string)
	url, _ := data["url"].(string)
	return strings.TrimSpace(mid) != "" || strings.TrimSpace(url) != ""
}

// permuteGalleryBlocks Fisher–Yates-shuffles gallery_image blocks that have media.
// Empty images are dropped. Other block types are omitted (lookbook is images-only).
func permuteGalleryBlocks(blocks []cms.Block, seed int64) []cms.Block {
	out := make([]cms.Block, 0, len(blocks))
	for _, b := range blocks {
		if b.Type != cms.BlockGalleryImage {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(b.Data, &data)
		if !galleryDataHasMedia(data) {
			continue
		}
		out = append(out, b)
	}
	fisherYates(out, seed)
	return out
}

func fisherYates(items []cms.Block, seed int64) {
	if len(items) < 2 {
		return
	}
	r := rand.New(rand.NewSource(seed))
	for i := len(items) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		items[i], items[j] = items[j], items[i]
	}
}
