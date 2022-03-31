package voronoi

import (
	"image"
	"math"
)

// Stolen from https://github.com/kellydunn/golang-geo/blob/master/polygon.go

// A Polygon is carved out of a 2D plane by a set of (possibly disjoint) contours.
// It can thus contain holes, and can be self-intersecting.
type Polygon struct {
	Points []image.Point
}

// NewPolygon: Creates and returns a new pointer to a Polygon
// composed of the passed in Points.  Points are
// considered to be in order such that the last point
// forms an edge with the first point.
func NewPolygon(Points []image.Point) *Polygon {
	return &Polygon{Points: Points}
}

// Bounds returns the highest & lowest x & y values from the Points in this polygon.
func (p *Polygon) Bounds() image.Rectangle {
	if len(p.Points) == 0 {
		return image.Rect(0, 0, 0, 0)
	}

	first := p.Points[0]

	x0 := first.X
	y0 := first.Y
	x1 := first.X
	y1 := first.Y

	for i := 1; i < len(p.Points); i++ {
		p := p.Points[i]
		if p.X < x0 {
			x0 = p.X
		} else if p.X > x1 {
			x1 = p.X
		}

		if p.Y < y0 {
			y0 = p.Y
		} else if p.Y > y1 {
			y1 = p.Y
		}
	}

	return image.Rect(x0, y0, x1, y1)
}

// IsClosed returns whether or not the polygon is closed.
// TODO:  This can obviously be improved, but for now,
//        this should be sufficient for detecting if Points
//        are contained using the raycast algorithm.
func (p *Polygon) IsClosed() bool {
	if len(p.Points) < 3 {
		return false
	}
	return true
}

// Contains returns whether or not the current Polygon contains the passed in Point.
// Nb. this is something of a fast approximation.
func (p *Polygon) Contains(point image.Point) bool {
	if !p.IsClosed() {
		return false
	}

	start := len(p.Points) - 1
	end := 0

	contains := p.intersectsWithRaycast(point, p.Points[start], p.Points[end])

	for i := 1; i < len(p.Points); i++ {
		if p.intersectsWithRaycast(point, p.Points[i-1], p.Points[i]) {
			contains = !contains
		}
	}

	return contains
}

// Using the raycast algorithm, this returns whether or not the passed in point
// Intersects with the edge drawn by the passed in start and end Points.
// Original implementation: http://rosettacode.org/wiki/Ray-casting_algorithm#Go
func (p *Polygon) intersectsWithRaycast(point, start, end image.Point) bool {
	// Always ensure that the the first point
	// has a y coordinate that is less than the second point
	if start.Y > end.Y {
		// Switch the Points if otherwise.
		start, end = end, start
	}

	// Move the point's y coordinate
	// outside of the bounds of the testing region
	// so we can start drawing a ray
	for point.Y == start.Y || point.Y == end.Y {
		newLng := int(math.Ceil(math.Nextafter(float64(point.Y), math.Inf(1))))
		point = image.Pt(point.X, newLng)
	}

	// If we are outside of the polygon, indicate so.
	if point.Y < start.Y || point.Y > end.Y {
		return false
	}

	if start.X > end.X {
		if point.X > start.X {
			return false
		}
		if point.X < end.X {
			return true
		}

	} else {
		if point.X > end.X {
			return false
		}
		if point.X < start.X {
			return true
		}
	}

	raySlope := float64(point.Y-start.Y) / float64(point.X-start.X)
	diagSlope := float64(end.Y-start.Y) / float64(end.X-start.X)

	return raySlope >= diagSlope
}
