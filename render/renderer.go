/*
Copyright (c) 2017 Lauris Bukšis-Haberkorns <lauris@nix.lv>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package render

import (
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Tsukumogami-Software/go-tiled"
	"github.com/hajimehoshi/ebiten/v2"
)

var (
	// ErrUnsupportedOrientation represents an error in the unsupported orientation for rendering.
	ErrUnsupportedOrientation = errors.New("tiled/render: unsupported orientation")
	// ErrUnsupportedRenderOrder represents an error in the unsupported order for rendering.
	ErrUnsupportedRenderOrder = errors.New("tiled/render: unsupported render order")

	// ErrOutOfBounds represents an error that the index is out of bounds
	ErrOutOfBounds = errors.New("tiled/render: index out of bounds")
)

// RendererEngine is the interface implemented by objects that provide rendering engine for Tiled maps.
type RendererEngine interface {
	Init(m *tiled.Map)
	GetFinalImageSize() (int, int)
	RotateTileImage(tile *tiled.LayerTile, img *ebiten.Image) *ebiten.Image
	GetTilePosition(x, y int) ebiten.GeoM
}

// Renderer represents an rendering engine.
type Renderer struct {
	m         *tiled.Map
	Result    *ebiten.Image // The image result after rendering using the Render functions.
	tileCache map[uint32]image.Image
	engine    RendererEngine
	fs        fs.FS
}

// NewRenderer creates new rendering engine instance.
func NewRenderer(m *tiled.Map) (*Renderer, error) {
	return NewRendererWithFileSystem(m, nil)
}

// NewRendererWithFileSystem creates new rendering engine instance with a custom file system.
func NewRendererWithFileSystem(m *tiled.Map, fs fs.FS) (*Renderer, error) {
	r := &Renderer{m: m, tileCache: make(map[uint32]image.Image), fs: fs}
	if r.m.Orientation == "orthogonal" {
		r.engine = &OrthogonalRendererEngine{}
	} else {
		return nil, ErrUnsupportedOrientation
	}

	r.engine.Init(r.m)
	r.Clear()

	return r, nil
}

func (r *Renderer) open(f string) (io.ReadCloser, error) {
	if r.fs == nil {
		return os.Open(filepath.FromSlash(f))
	}
	return r.fs.Open(filepath.ToSlash(f))
}

func (r *Renderer) getTileImageFromTile(tile *tiled.LayerTile) (*ebiten.Image, error) {
		tilesetTile, err := tile.Tileset.GetTilesetTile(tile.ID)
		if err != nil {
			return nil, err
		}

		sf, err := r.open(tile.Tileset.GetFileFullPath(tilesetTile.Image.Source))
		if err != nil {
			return nil, err
		}
		defer sf.Close()

		img, _, err := image.Decode(sf)
		if err != nil {
			return nil, err
		}

		timg := ebiten.NewImageFromImage(img)
		r.tileCache[tile.Tileset.FirstGID+tile.ID] = timg
		return r.engine.RotateTileImage(tile, timg), nil
}

func (r *Renderer) getTileImageFromTileset(tile *tiled.LayerTile) (*ebiten.Image, error) {
	sf, err := r.open(tile.Tileset.GetFileFullPath(tile.Tileset.Image.Source))
	if err != nil {
		return nil, err
	}
	defer sf.Close()

	img, _, err := image.Decode(sf)
	if err != nil {
		return nil, err
	}
	eimg := ebiten.NewImageFromImage(img)

	// Precache all tiles in tileset
	var timg *ebiten.Image
	for i := uint32(0); i < uint32(tile.Tileset.TileCount); i++ {
		rect := tile.Tileset.GetTileRect(i)
		r.tileCache[i+tile.Tileset.FirstGID] = eimg.SubImage(rect)
		if tile.ID == i {
			timg = ebiten.NewImageFromImage(r.tileCache[i+tile.Tileset.FirstGID])
		}
	}

	if timg != nil {
		return r.engine.RotateTileImage(tile, timg), nil
	}
	return nil, errors.New(
		fmt.Sprintf("Tile image not found in tileset: %d", tile.ID),
	)
}

func (r *Renderer) getTileImage(tile *tiled.LayerTile) (*ebiten.Image, error) {
	timg, ok := r.tileCache[tile.Tileset.FirstGID+tile.ID]
	if ok {
		res := ebiten.NewImageFromImage(timg)
		return r.engine.RotateTileImage(tile, res), nil
	}

	if tile.Tileset.Image == nil {
		return r.getTileImageFromTile(tile)
	}

	return r.getTileImageFromTileset(tile)
}

func (r *Renderer) _renderLayer(layer *tiled.Layer) error {
	var xs, xe, xi, ys, ye, yi int
	if r.m.RenderOrder == "" || r.m.RenderOrder == "right-down" {
		xs = 0
		xe = r.m.Width
		xi = 1
		ys = 0
		ye = r.m.Height
		yi = 1
	} else {
		return ErrUnsupportedRenderOrder
	}

	i := 0
	for y := ys; y*yi < ye; y = y + yi {
		for x := xs; x*xi < xe; x = x + xi {
			if layer.Tiles[i].IsNil() {
				i++
				continue
			}

			img, err := r.getTileImage(layer.Tiles[i])
			if err != nil {
				return err
			}

			geom := r.engine.GetTilePosition(x, y)

			colorScale := ebiten.ColorScale{}
			colorScale.SetA(layer.Opacity)

			r.Result.DrawImage(img, &ebiten.DrawImageOptions{
				GeoM: geom,
				ColorScale: colorScale,
			})

			i++
		}
	}

	return nil
}

// RenderGroupLayer renders single map layer in a certain group.
func (r *Renderer) RenderGroupLayer(groupID, layerID int) error {
	if groupID >= len(r.m.Groups) {
		return ErrOutOfBounds
	}
	group := r.m.Groups[groupID]

	if layerID >= len(group.Layers) {
		return ErrOutOfBounds
	}
	return r._renderLayer(group.Layers[layerID])
}

// RenderLayer renders single map layer.
func (r *Renderer) RenderLayer(id int) error {
	if id >= len(r.m.Layers) {
		return ErrOutOfBounds
	}
	return r._renderLayer(r.m.Layers[id])
}

// RenderVisibleLayers renders all visible map layers.
func (r *Renderer) RenderVisibleLayers() error {
	for i := range r.m.Layers {
		if !r.m.Layers[i].Visible {
			continue
		}

		if err := r.RenderLayer(i); err != nil {
			return err
		}
	}

	return nil
}

// Clear clears the render result to allow for separation of layers. For example, you can
// render a layer, make a copy of the render, clear the renderer, and repeat for each
// layer in the Map.
func (r *Renderer) Clear() {
	width, height := r.engine.GetFinalImageSize()
	r.Result = ebiten.NewImage(width, height)
}

// SaveAsPng writes rendered layers as PNG image to provided writer.
func (r *Renderer) SaveAsPng(w io.Writer) error {
	return png.Encode(w, r.Result)
}

// SaveAsJpeg writes rendered layers as JPEG image to provided writer.
func (r *Renderer) SaveAsJpeg(w io.Writer, options *jpeg.Options) error {
	return jpeg.Encode(w, r.Result, options)
}

// SaveAsGif writes rendered layers as GIF image to provided writer.
func (r *Renderer) SaveAsGif(w io.Writer, options *gif.Options) error {
	return gif.Encode(w, r.Result, options)
}
