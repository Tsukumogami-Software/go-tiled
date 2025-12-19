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
	"image"
	"math"

	"github.com/Tsukumogami-Software/go-tiled"
	"github.com/Tsukumogami-Software/go-tiled/internal/utils"
	"github.com/hajimehoshi/ebiten/v2"
)

// RenderVisibleGroups renders all visible groups
func (r *Renderer) RenderVisibleGroups() error {
	for _, group := range r.m.Groups {
		if !group.Visible {
			continue
		}
		if err := r._renderGroup(group); err != nil {
			return err
		}
	}
	return nil
}

// RenderGroup renders single group.
func (r *Renderer) RenderGroup(groupID int) error {
	if groupID >= len(r.m.Groups) {
		return ErrOutOfBounds
	}

	group := r.m.Groups[groupID]
	return r._renderGroup(group)
}

func (r *Renderer) _renderGroup(group *tiled.Group) error {
	for _, layer := range group.Layers {
		if !layer.Visible {
			continue
		}
		if err := r._renderLayer(layer); err != nil {
			return err
		}
	}

	for _, objectGroup := range group.ObjectGroups {
		if !objectGroup.Visible {
			continue
		}
		if err := r._renderObjectGroup(objectGroup); err != nil {
			return err
		}
	}

	return nil
}

// RenderVisibleLayersAndObjectGroups render all layers and object groups, layer first, objectGroup second
// so the order may be incorrect,
// you may put them into different groups, then call RenderVisibleGroups
func (r *Renderer) RenderVisibleLayersAndObjectGroups() error {
	// TODO: The order maybe incorrect

	if err := r.RenderVisibleLayers(); err != nil {
		return err
	}
	return r.RenderVisibleObjectGroups()
}

// RenderVisibleObjectGroups renders all visible object groups
func (r *Renderer) RenderVisibleObjectGroups() error {
	for i, layer := range r.m.ObjectGroups {
		if !layer.Visible {
			continue
		}
		if err := r.RenderObjectGroup(i); err != nil {
			return err
		}
	}
	return nil
}

// RenderObjectGroup renders a single object group
func (r *Renderer) RenderObjectGroup(i int) error {
	if i >= len(r.m.ObjectGroups) {
		return ErrOutOfBounds
	}

	layer := r.m.ObjectGroups[i]
	return r._renderObjectGroup(layer)
}

func (r *Renderer) _renderObjectGroup(objectGroup *tiled.ObjectGroup) error {
	objs := objectGroup.Objects

	// sort objects from left top to right down
	objs = utils.SortAnySlice(objs, func(a, b *tiled.Object) bool {
		if a.Y != b.Y {
			return a.Y < b.Y
		}

		return a.X < b.X
	})

	for _, obj := range objs {
		if err := r.renderOneObject(objectGroup, obj); err != nil {
			return err
		}
	}
	return nil
}

// RenderGroupObjectGroup renders single object group in a certain group.
func (r *Renderer) RenderGroupObjectGroup(groupID, objectGroupID int) error {
	if groupID >= len(r.m.Groups) {
		return ErrOutOfBounds
	}

	group := r.m.Groups[groupID]

	if objectGroupID >= len(group.ObjectGroups) {
		return ErrOutOfBounds
	}

	layer := group.ObjectGroups[objectGroupID]
	return r._renderObjectGroup(layer)
}

func (r *Renderer) renderOneObject(layer *tiled.ObjectGroup, o *tiled.Object) error {
	if !o.Visible {
		return nil
	}

	if o.GID == 0 {
		// TODO: o.GID == 0
		return nil
	}

	tile, err := r.m.TileGIDToTile(o.GID)
	if err != nil {
		return err
	}

	img, err := r.getTileImage(tile)
	if err != nil {
		return err
	}

	geom := ebiten.GeoM{}

	bounds := img.Bounds()
	srcSize := bounds.Size()
	dstSize := image.Pt(int(o.Width), int(o.Height))

	if !srcSize.Eq(dstSize) {
		geom.Scale(
			float64(dstSize.X)/float64(srcSize.X),
			float64(dstSize.Y)/float64(srcSize.Y),
		)
	}

	if o.Rotation != 0 {
		geom.Rotate(o.Rotation * math.Pi / 180.0)
	}

	colorScale := ebiten.ColorScale{}
	colorScale.SetA(layer.Opacity)

	r.Result.DrawImage(
		img.(*ebiten.Image),
		&ebiten.DrawImageOptions{
			GeoM:       geom,
			ColorScale: colorScale,
		})

	return nil
}
