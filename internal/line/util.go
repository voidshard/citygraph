package line

import (
	"image"
	"image/color"
)

// listPlot meets the Plotter interface,
// In our case we just append the x,y to a list,
type listPlot struct {
	pts []image.Point
}

// Set records a new point on the line
func (l *listPlot) Set(x, y int, c color.Color) {
	l.pts = append(l.pts, image.Pt(x, y))
}

// PointsBetween returns all points on a line between a,b
func PointsBetween(a, b image.Point) []image.Point {
	lp := &listPlot{pts: []image.Point{}}
	bresenham(lp, a.X, a.Y, b.X, b.Y, color.Black)
	return lp.pts
}
