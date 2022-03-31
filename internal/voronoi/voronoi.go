package voronoi

import (
	"image"
	"os"
	"path/filepath"

	"github.com/unixpickle/model3d/model2d"
)

// Voronoi wraps a quasoft.Voronoi
type Voronoi struct {
	vg     VoronoiDiagram
	sites  []Site
	bounds image.Rectangle
}

// newVoronoi builds a voronoi diagram using the given builder information
func newVoronoi(b *Builder) *Voronoi {
	me := &Voronoi{bounds: b.bounds}

	points := make([]model2d.Coord, len(b.sites))
	for i, s := range b.sites {
		points[i] = model2d.Coord{float64(s.X), float64(s.Y)}
	}

	me.vg = VoronoiCells(
		model2d.Coord{float64(b.bounds.Min.X), float64(b.bounds.Min.Y)},
		model2d.Coord{float64(b.bounds.Max.X), float64(b.bounds.Max.Y)},
		points,
	)
	me.vg.Repair(1e-8)

	me.sites = make([]Site, len(me.vg))
	for i, cell := range me.vg {
		me.sites[i] = &vSite{id: i, cell: cell, parent: me}
	}

	return me
}

// Bounds returns the bounding rect for this diagram
func (v *Voronoi) Bounds() image.Rectangle {
	return v.bounds
}

// Sites returns all sites
func (v *Voronoi) Sites() []Site {
	return v.sites
}

// SiteByID returns the given Site by it's ID
func (v *Voronoi) SiteByID(i int) Site {
	if i < 0 || i >= len(v.sites) {
		return nil
	}
	return v.sites[i]
}

// SiteFor returns the nearest Site ("centre" of a voronoi cell) for the given point.
func (v *Voronoi) SiteFor(x, y int) Site {
	dist := -1.0
	var pick Site
	for _, site := range v.sites {
		sdist := calculateDist(site.X(), site.Y(), x, y)
		if dist == 0 {
			return site
		} else if dist < 0 || sdist < dist {
			dist = sdist
			pick = site
		}
	}
	return pick
}

// DebugRender writes to os.TempDir "voronoi.png"
func (v *Voronoi) DebugRender() error {
	fpath := filepath.Join(os.TempDir(), "voronoi.png")
	return v.vg.Render(fpath)
}
