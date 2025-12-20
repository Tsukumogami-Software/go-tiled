package render

import (
	"image"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Tsukumogami-Software/go-tiled"
	"github.com/hajimehoshi/ebiten/v2"
)

type TilesetCache struct {
	cache map[string]map[uint32]image.Image
	fs fs.FS
}

func NewTilesetCache(fs fs.FS) *TilesetCache {
	return &TilesetCache{
		cache: map[string]map[uint32]image.Image{},
		fs: fs,
	}
}

func (t *TilesetCache) open(f string) (io.ReadCloser, error) {
	if t.fs == nil {
		return os.Open(filepath.FromSlash(f))
	}
	return t.fs.Open(filepath.ToSlash(f))
}

func (t *TilesetCache) cacheTileset(tileset *tiled.Tileset) error {
	sf, err := t.open(tileset.GetFileFullPath(tileset.Image.Source))
	if err != nil {
		return err
	}
	defer sf.Close()

	img, _, err := image.Decode(sf)
	if err != nil {
		return err
	}
	eimg := ebiten.NewImageFromImage(img)

	cache := make(map[uint32]image.Image, tileset.TileCount)
	for i := uint32(0); i < uint32(tileset.TileCount); i++ {
		rect := tileset.GetTileRect(i)
		cache[i] = eimg.SubImage(rect)
	}

	t.cache[tileset.Name] = cache
	return nil
}

// GetTileImage finds a SubImage from cache
func (t *TilesetCache) GetTileImage(tile *tiled.LayerTile) (image.Image, error) {
	cached, ok := t.cache[tile.Tileset.Name]
	if !ok {
		err := t.cacheTileset(tile.Tileset)
		if err != nil {
			return nil, err
		}
		cached = t.cache[tile.Tileset.Name]
	}

	return cached[tile.ID], nil
}
