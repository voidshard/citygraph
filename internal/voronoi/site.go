package voronoi

import (
	"fmt"
	"image"
	"math"
)

// Site exposes useful functions of a voronoi diagram Site
type Site interface {
	ID() int
	X() int
	Y() int

	Edges() [][2]image.Point
	Vertices() []image.Point
	Contains(x, y int) bool
	AllContains() PointGenerator
	Bounds() image.Rectangle
	Neighbours() []*Neighbour
}

// PointGenerator returns all points within a Site, in a somewhat
// sane fashion that doesn't involve pre-computing a huge set
type PointGenerator interface {
	Next() *image.Point
}

// vSite is a wrapper around Voronoi & VoronoiCell
type vSite struct {
	id     int
	parent *Voronoi
	cell   *VoronoiCell
	poly   *Polygon
	edges  [][2]image.Point
}

// vPGen satisties PointGenerator
type vPGen struct {
	bounds image.Rectangle
	poly   *Polygon
	x      int
	y      int
}

// Next return the next point contained in a Site.
// A nil value indicates that there are no more.
func (v *vPGen) Next() *image.Point {
	if v.x == -1 && v.y == -1 {
		v.x = v.bounds.Min.X
		v.y = v.bounds.Min.Y
	}

	for ; v.y < v.bounds.Max.Y; v.y++ {
		for x := v.x; x < v.bounds.Max.X; x++ {
			p := image.Pt(x, v.y)
			if v.poly.Contains(p) {
				v.x = x + 1
				return &p
			}
		}
		v.x = v.bounds.Min.X
	}

	return nil
}

// AllContains returns a PointGenerator for the given site
func (s *vSite) AllContains() PointGenerator {
	return &vPGen{
		bounds: s.Bounds(),
		poly:   s.poly,
		x:      -1,
		y:      -1,
	}
}

// Neighbour is a Site & Edge that is shared by another cell.
// See Neighbours()
type Neighbour struct {
	Site  Site
	Edges [][2]image.Point
}

// Neighbours returns all Sites that share an edge with this site.
func (s *vSite) Neighbours() []*Neighbour {
	toEdgeId := func(e [2]image.Point) string {
		a, b := e[0], e[1]
		if b.X < a.X {
			a, b = b, a
		}
		if a.X == b.X && b.Y < a.Y {
			a, b = b, a
		}
		return fmt.Sprintf("%d.%d-%d.%d", a.X, a.Y, b.X, b.Y)
	}

	myedges := map[string][2]image.Point{}
	for _, e := range s.Edges() {
		myedges[toEdgeId(e)] = e
	}

	ls := []*Neighbour{}
	for _, syte := range s.parent.Sites() {
		if syte.ID() == s.ID() {
			continue
		}

		n := &Neighbour{Site: syte, Edges: [][2]image.Point{}}
		for _, e := range syte.Edges() {
			eid := toEdgeId(e)
			_, ok := myedges[eid]
			if !ok {
				continue
			}
			n.Edges = append(n.Edges, e)
		}
		if len(n.Edges) > 0 {
			ls = append(ls, n)
		}
	}

	return ls
}

// ID of this site
func (s *vSite) ID() int {
	return s.id
}

// X value of site centre
func (s *vSite) X() int {
	return int(s.cell.Center.X)
}

// Y value of site centre
func (s *vSite) Y() int {
	return int(s.cell.Center.Y)
}

// buildPolygon constructs a polygon from the veticies surrounding the site
func (s *vSite) buildPolygon() {
	s.poly = &Polygon{Points: []image.Point{}}
	s.edges = [][2]image.Point{}

	for _, edge := range s.cell.Edges {
		start := image.Pt(int(math.Round(edge[0].X)), int(math.Round(edge[0].Y)))
		end := image.Pt(int(math.Round(edge[1].X)), int(math.Round(edge[1].Y)))

		s.poly.Points = append(s.poly.Points, start)
		s.edges = append(s.edges, [2]image.Point{start, end})
	}
}

// Edges returns all edges surrounding this site
func (s *vSite) Edges() [][2]image.Point {
	if s.poly == nil {
		s.buildPolygon()
	}
	return s.edges
}

// Vertices returns all vertexes (through which edges pass) of the site
func (s *vSite) Vertices() []image.Point {
	if s.poly == nil {
		s.buildPolygon()
	}
	return s.poly.Points
}

// Contains returns if this site contains x,y .. this is a rough
// approximation for quick calculations.
func (s *vSite) Contains(x, y int) bool {
	if s.poly == nil {
		s.buildPolygon()
	}
	return s.poly.Contains(image.Pt(x, y))
}

// Bounds returns a rectangle that necessarily contains all points in the site
// and more besides.
func (s *vSite) Bounds() image.Rectangle {
	if s.poly == nil {
		s.buildPolygon()
	}
	return s.poly.Bounds()
}
