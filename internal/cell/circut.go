package cell

import (
	"fmt"
	"image"

	"github.com/voidshard/citygraph/internal/voronoi"
)

// Circut finds a path around all "inside" site(s).
// Notes.
// 1. the behavoir of this is undefined if the same site is in both inside & outside
// 2. the points returned are in no particular order
// 3. we consider points on the edges of "bounds" to be important
//    ie, a site "inside" in a corner will have edges travelling along the borders
func Circut(bnds image.Rectangle, inside, outside []voronoi.Site) [][2]image.Point {
	return deletionMethod(bnds, inside, outside)
}

// deletionMethod attempts to build the above circut via a "deletion" strategy.
// Essentially we start with every edge of an "inside" site being considered valid.
// We then look to remove all edges whose edge (& verticies making the edge) are not
// also shared by site(s) in the "outside" sites.
// The idea here is that all edges / verts along the edge of "inside" sites are
// by definiton either edges / verts of "outside" sites OR are along a border.
func deletionMethod(bnds image.Rectangle, inside, outside []voronoi.Site) [][2]image.Point {
	// nb. we could probably make this more efficient

	onBorder := func(p image.Point) bool {
		// we also count the next closest point the edge as the edge
		// ie. 1, 999 in 0-1000
		// .. because the voronoi graph rounds floats & can have rounding errors ..
		return p.X <= bnds.Min.X+1 || p.X >= bnds.Max.X-1 || p.Y <= bnds.Min.Y+1 || p.Y >= bnds.Max.Y-1
	}

	toEdgeID := func(a, b image.Point) string {
		if b.X < a.X {
			a, b = b, a
		} else if a.X == b.X && b.Y < a.Y {
			a, b = b, a
		}
		return fmt.Sprintf("%d,%d-%d,%d", a.X, a.Y, b.X, b.Y)
	}

	edgeOwnedOutside := map[string]bool{}
	vertOwnedOutside := map[image.Point]bool{}
	for _, s := range outside {
		for _, edge := range s.Edges() {
			vertOwnedOutside[edge[0]] = true
			vertOwnedOutside[edge[1]] = true
			edgeOwnedOutside[toEdgeID(edge[0], edge[1])] = true
		}
	}

	neighbours := map[image.Point]map[image.Point]bool{}
	for _, s := range inside {
		for _, edge := range s.Edges() {
			next, ok := neighbours[edge[0]]
			if !ok {
				next = map[image.Point]bool{}
			}
			next[edge[1]] = true
			neighbours[edge[0]] = next

			next, ok = neighbours[edge[1]]
			if !ok {
				next = map[image.Point]bool{}
			}
			next[edge[0]] = true
			neighbours[edge[1]] = next
		}
	}

	for v, partners := range neighbours {
		vOutside, _ := vertOwnedOutside[v]
		for n, valid := range partners {
			if !valid {
				continue
			}

			eOutside, _ := edgeOwnedOutside[toEdgeID(v, n)]
			nOutside, _ := vertOwnedOutside[n]
			if vOutside && nOutside && eOutside {
				continue
			}
			if onBorder(n) && onBorder(v) {
				continue
			}

			neighbours[n][v] = false
			neighbours[v][n] = false
		}
	}

	wall := [][2]image.Point{}
	for vertex, partners := range neighbours {
		for end, valid := range partners {
			if !valid {
				continue
			}
			wall = append(wall, [2]image.Point{vertex, end})
		}
	}

	return wall
}
