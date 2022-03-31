package voronoi

import (
	"image/color"
	"math"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/model3d/model2d"
	"github.com/unixpickle/model3d/model3d"
)

// stolen from
// https://github.com/unixpickle/voronoi-glass/blob/main/voronoi.go
//
// + some improvements re. edge ordering etc etc

type VoronoiCell struct {
	Center model2d.Coord
	Edges  []*model2d.Segment
}

type VoronoiDiagram []*VoronoiCell

// VoronoiCells computes the voronoi cells for a list of
// coordinates, assuming they are all contained within a
// bounding box.
//
// The resulting Voronoi cells may be slightly misaligned,
// i.e. adjacent edges' coordinates may differ due to
// rounding errors. See VoronoiDiagram.Repair().
func VoronoiCells(min, max model2d.Coord, coords []model2d.Coord) VoronoiDiagram {
	cells := make([]*VoronoiCell, len(coords))
	for i, c := range coords {
		constraints := model2d.NewConvexPolytopeRect(min, max)
		for _, c1 := range coords {
			if c != c1 {
				mp := c.Mid(c1)
				normal := c1.Sub(c).Normalize()
				constraint := &model2d.LinearConstraint{
					Normal: normal,
					Max:    normal.Dot(mp),
				}
				constraints = append(constraints, constraint)
			}
		}
		cells[i] = &VoronoiCell{
			Center: c,
			Edges:  constraints.Mesh().SegmentSlice(),
		}
	}
	return cells
}

// Repair merges nearly identical coordinates to make a
// well-connected graph.
func (v VoronoiDiagram) Repair(epsilon float64) {
	coordSet := map[model2d.Coord]bool{}
	coordSlice := []model2d.Coord{}
	for _, cell := range v {
		for _, s := range cell.Edges {
			for _, p := range s {
				if !coordSet[p] {
					coordSet[p] = true
					coordSlice = append(coordSlice, p)
				}
			}
		}
	}
	tree := model2d.NewCoordTree(coordSlice)

	mapping := map[model2d.Coord]model2d.Coord{}
	for _, c := range coordSlice {
		if !coordSet[c] {
			continue
		}
		neighbors := neighborsInDistance(tree, c, epsilon)
		for _, n := range neighbors {
			if coordSet[n] {
				coordSet[n] = false
				mapping[n] = c
			}
		}
	}

	for _, cell := range v {
		starts := map[model2d.Coord]*model2d.Segment{}

		for i := 0; i < len(cell.Edges); i++ {
			edge := cell.Edges[i]
			for j, c := range edge {
				edge[j] = mapping[c]
			}
			if edge[0] == edge[1] {
				// This was almost a singular edge.
				essentials.UnorderedDelete(&cell.Edges, i)
			} else {
				starts[edge[0]] = edge
			}
		}

		// now to sort the edges
		newOrder := make([]*model2d.Segment, len(cell.Edges))
		newOrder[0] = cell.Edges[0]

		for i := 0; i < len(cell.Edges)-1; i++ {
			prev := newOrder[i]
			next, _ := starts[prev[1]]
			newOrder[i+1] = next
		}

		cell.Edges = newOrder // same edges, re-ordered
	}
}

func (v VoronoiDiagram) Coords() []model2d.Coord {
	coordSet := map[model2d.Coord]bool{}
	coordSlice := []model2d.Coord{}
	for _, cell := range v {
		for _, s := range cell.Edges {
			for _, p := range s {
				if !coordSet[p] {
					coordSet[p] = true
					coordSlice = append(coordSlice, p)
				}
			}
		}
	}
	return coordSlice
}

func (v VoronoiDiagram) Render(path string) error {
	mesh2d := model2d.NewMesh()
	for _, cell := range v {
		mesh2d.AddMesh(model2d.NewMeshSegments(cell.Edges))
	}
	size := mesh2d.Max().Sub(mesh2d.Min())
	maxSize := math.Max(size.X, size.Y)

	pointsSolid := model2d.JoinedSolid{}
	for _, cell := range v {
		pointsSolid = append(pointsSolid, &model2d.Circle{
			Center: cell.Center,
			Radius: math.Max(2, maxSize/200),
		})
	}

	bg := model2d.NewRect(mesh2d.Min(), mesh2d.Max())
	return model2d.RasterizeColor(path, []interface{}{
		bg,
		model2d.IntersectedSolid{pointsSolid.Optimize(), bg},
		mesh2d,
	}, []color.Color{
		color.Gray{Y: 0xff},
		color.RGBA{B: 0xff, A: 0xff},
		color.RGBA{R: 0xff, A: 0xff},
	}, 1.0)
}

func (v VoronoiDiagram) Mesh() *model3d.Mesh {
	mesh := model3d.NewMesh()
	for _, cell := range v {
		mesh2d := model2d.NewMeshSegments(cell.Edges)
		triangles := model2d.TriangulateMesh(mesh2d)
		for _, t := range triangles {
			t3d := &model3d.Triangle{}
			for i, c := range t {
				t3d[i] = model3d.XYZ(c.X, c.Y, 0)
			}
			mesh.Add(t3d)
		}
	}
	return mesh
}

func neighborsInDistance(tree *model2d.CoordTree, c model2d.Coord, epsilon float64) []model2d.Coord {
	for k := 2; true; k++ {
		neighbors := tree.KNN(k, c)
		if len(neighbors) < k {
			return neighbors
		}
		if neighbors[len(neighbors)-1].Dist(c) > epsilon {
			return neighbors[:len(neighbors)-1]
		}
	}
	panic("unreachable")
}
