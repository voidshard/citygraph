package citygraph

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"math"
	"math/rand"
	"sort"

	"github.com/voidshard/citygraph/internal/voronoi"
)

// gateLocation encodes where we might put a gatehouse with associated
// towers, walls, etc etc etc
type gateLocation struct {
	In      voronoi.Site
	InDist  *District
	Out     voronoi.Site
	OutDist *District
	Edge    [2]image.Point

	// locations of stuff of interest
	Towers    []image.Rectangle
	Gatehouse image.Rectangle
	Walls     [][2]image.Point

	// the left & right points where we "cut" the wall
	Left  image.Point
	Right image.Point
}

// withinGateCourtyard returns if the given point is directly infront of the gate
// within the wall indentation
func (g *gateLocation) withinGateCourtyard(in image.Point) bool {
	a, b := g.Left, g.Right
	if in.X < a.X || in.X > b.X {
		return false
	}

	if b.Y < a.Y {
		a, b = b, a
	}
	if in.Y < a.Y || in.Y > b.Y {
		return false
	}

	return true
}

// determinePlacements figures out where the towers / gate / walls for the gatehouse
// should be (if possible). This doesn't check if these sit on valid pixels.
func (g *gateLocation) determinePlacements(graph *voronoi.Voronoi, tower, gate image.Rectangle) error {
	tw := tower.Max.X - tower.Min.X
	th := tower.Max.Y - tower.Min.Y
	gw := gate.Max.X - gate.Min.X
	gh := gate.Max.Y - gate.Min.Y

	// we want to stick a gatehouse in the edge centre, so we
	// have some math to work out it's position, orientation
	// and how it's indented into the wall.
	// We also add 4 towers - one either side of the gate and one
	// on either side of the "indent" we put into the wall.

	a, b := g.Edge[0], g.Edge[1]
	if b.X < a.X {
		a, b = b, a
	}
	dx := float64(b.X - a.X)
	dy := float64(b.Y - a.Y)

	m := 0.0     // gradient
	if dx != 0 { // rise over run for y = mx + c (equation of line)
		m = dy / dx
	}
	c := float64(a.Y) - m*float64(a.X) // y - mx = c

	middle := image.Pt((a.X+b.X)/2, (a.Y+b.Y)/2) // mid point on edge

	vert := math.Abs(dy) > math.Abs(dx) // if gatehouse should be vertical or horizontal

	mult := 1
	if m < 0 { // indicates gradient is negative or not
		mult = -1
	}

	if vert { // gatehouse aligned vertically x = (y-c)/m
		// pass Y values, solve for X
		total := float64(th + th + gh)
		ly := float64(middle.Y) - total/2
		lx := (ly - c) / m
		ry := float64(middle.Y) + total/2
		rx := (ry - c) / m
		left := image.Pt(int(lx), int(ly))
		right := image.Pt(int(rx), int(ry))

		if right.X < left.X {
			left, right = right, left
		}

		// centre of two straight lines on either side,
		// we'll use whichever is inside our site
		imid := image.Pt(left.X+-1*mult*2*tw, middle.Y) // check left
		if graph.SiteFor(imid.X, imid.Y).ID() != g.In.ID() {
			imid = image.Pt(right.X+mult*2*tw, middle.Y) // check right
			if graph.SiteFor(imid.X, imid.Y).ID() != g.In.ID() {
				// probably the site is too narrow for indent
				return fmt.Errorf("unable to fit gatehouse")
			}
		}

		g.Towers = []image.Rectangle{
			image.Rect(left.X-tw/2, left.Y-th/2, left.X+tw/2, left.Y+th/2),
			image.Rect(right.X-tw/2, right.Y-th/2, right.X+tw/2, right.Y+th/2),
			image.Rect(imid.X-tw/2, imid.Y-int(total/2)-th/2, imid.X+tw/2, imid.Y-int(total/2)+th/2),
			image.Rect(imid.X-tw/2, imid.Y+int(total/2)-th/2, imid.X+tw/2, imid.Y+int(total/2)+th/2),
		}
		g.Gatehouse = image.Rect(imid.X-gw/2, imid.Y-gh/2, imid.X+gw/2, imid.Y+gh/2)
		g.Walls = [][2]image.Point{
			[2]image.Point{left, image.Pt(imid.X, imid.Y+int(total/2)*mult*-1)},
			[2]image.Point{right, image.Pt(imid.X, imid.Y+int(total/2)*mult)},
			[2]image.Point{
				image.Pt(imid.X, imid.Y-int(total/2)),
				image.Pt(imid.X, imid.Y+int(total/2)),
			},
		}
		g.Left = left
		g.Right = right
	} else { // gatehouse aligned horizontally y = mx+c
		// pass X values, solve for Y
		total := float64(tw + tw + gw)
		lx := float64(middle.X) - total/2
		ly := m*lx + c
		rx := float64(middle.X) + total/2
		ry := m*rx + c
		left := image.Pt(int(lx), int(ly))
		right := image.Pt(int(rx), int(ry))

		if right.X < left.X {
			left, right = right, left
		}

		imid := image.Pt(middle.X, left.Y+mult*2*th)
		if graph.SiteFor(imid.X, imid.Y).ID() != g.In.ID() {
			imid = image.Pt(middle.X, right.Y+-1*mult*2*th)
			if graph.SiteFor(imid.X, imid.Y).ID() != g.In.ID() {
				// probably the site is too narrow for indent
				return fmt.Errorf("unable to fit gatehouse")
			}
		}

		g.Towers = []image.Rectangle{
			image.Rect(left.X-tw/2, left.Y-th/2, left.X+tw/2, left.Y+th/2),
			image.Rect(right.X-tw/2, right.Y-th/2, right.X+tw/2, right.Y+th/2),
			image.Rect(imid.X-tw/2-int(total/2), imid.Y-th/2, imid.X+tw/2-int(total/2), imid.Y+th/2),
			image.Rect(imid.X-tw/2+int(total/2), imid.Y-th/2, imid.X+tw/2+int(total/2), imid.Y+th/2),
		}
		g.Gatehouse = image.Rect(imid.X-gw/2, imid.Y-gh/2, imid.X+gw/2, imid.Y+gh/2)
		g.Walls = [][2]image.Point{
			[2]image.Point{left, image.Pt(imid.X-int(total/2), imid.Y)},
			[2]image.Point{right, image.Pt(imid.X+int(total/2), imid.Y)},
			[2]image.Point{
				image.Pt(imid.X+-1*mult*int(total/2), imid.Y),
				image.Pt(imid.X+mult*int(total/2), imid.Y),
			},
		}
		g.Left = left
		g.Right = right
	}

	return nil
}

// simplified "buildingFits" as gates / towers are some of the first placed structures & so
// can save on quite a few checks for stuff that can't possibly exist yet.
func (g *gateLocation) fortificationsFit(cm CityMap, outline Outline) bool {
	fit := func(area image.Rectangle) bool {
		for x := area.Min.X; x < area.Max.X; x++ {
			for y := area.Min.Y; y < area.Max.Y; y++ {
				if cm.IsWall(x, y) || cm.IsTower(x, y) || cm.IsGatehouse(x, y) {
					return false
				}
				if outline.CanBuildOn(x, y) && (g.In.Contains(x, y) || g.Out.Contains(x, y)) {
					continue
				}
				return false
			}
		}
		return true
	}

	if !fit(g.Gatehouse) {
		return false
	}

	for _, tower := range g.Towers {
		if !fit(tower) {
			return false
		}
	}

	return true
}

// districtBuilder handles placing buildings -- essentially choosing a building
// footprint at random based on configuration / what size(s) fit etc
type districtBuilder struct {
	outline Outline
	d       *District
	cfg     *DistrictConfig
	site    voronoi.Site
	rng     *rand.Rand
	cm      CityMap

	total float64
	must  []*BuildingConfig
	count map[int]int
}

// newDistrictBuilder preps a builder to begin picking buildings.
func newDistrictBuilder(seed int64, o Outline, d *District, c *DistrictConfig, s voronoi.Site, m CityMap) *districtBuilder {
	db := &districtBuilder{
		outline: o,
		d:       d,
		cfg:     c,
		site:    s,
		total:   0.0,
		must:    []*BuildingConfig{},
		rng:     rand.New(rand.NewSource(seed + int64(d.ID))),
		count:   map[int]int{},
		cm:      m,
	}

	for _, bc := range c.Buildings {
		db.total += bc.Probability
		db.count[bc.ID] = 0

		for i := 0; i < bc.MinInDistrict; i++ {
			db.must = append(db.must, bc)
		}
	}

	return db
}

// chooseBuilding for the given x,y position (top left)
// We attempt to ensure that buildings with a Min number(s) are placed first.
// Max numbers are respected & probabilities of buildings used.
// Despite this it's often easier to place smaller buildings, so probabilities
// may wish to weight larger buildings slightly higher in general than strictly desired.
func (d *districtBuilder) chooseBuilding(x, y int) *BuildingConfig {
	for i, b := range d.must {
		// attempt to place buildings we *must* place first
		if !d.buildingFits(x, y, b) {
			continue
		}
		d.must = append(d.must[:i], d.must[i+1:]...)

		num, _ := d.count[b.ID]
		d.count[b.ID] = num + 1

		return b
	}

	rv := d.rng.Float64()
	sofar := 0.0

	// move on to generic random buildings
	for _, b := range d.cfg.Buildings {
		if b.Probability <= 0 {
			continue
		}

		sofar += b.Probability / d.total

		num, _ := d.count[b.ID]
		if b.MaxInDistrict > 0 && num >= b.MaxInDistrict {
			continue
		}
		if !d.buildingFits(x, y, b) {
			continue
		}

		if sofar > rv {
			d.count[b.ID] = num + 1
			return b
		}
	}

	return nil
}

// buildingFits returns if the building b fits at (ox,oy) (top left)
func (d *districtBuilder) buildingFits(ox, oy int, b *BuildingConfig) bool {
	for x := b.Area.Min.X; x < b.Area.Max.X; x++ {
		// nb. we pad top & bottom by 1 tile so buildings can't run together
		// top to bottom (but they can sit right next to each other in x terms)
		for y := b.Area.Min.Y - 1; y < b.Area.Max.Y+1; y++ {
			if d.cm.IsRoad(x+ox, y+oy) || d.cm.IsBridge(x+ox, y+oy) || d.cm.IsWall(x+ox, y+oy) || d.cm.IsTower(x+ox, y+oy) || d.cm.IsGatehouse(x+ox, y+oy) {
				return false
			}
			bID, _ := d.cm.BuildingID(x+ox, y+oy)
			if bID != 0 {
				return false
			}
			if d.outline.CanBuildOn(x+ox, y+oy) && d.site.Contains(x+ox, y+oy) {
				continue
			}
			return false
		}
	}
	return true
}

// calculateDist standard pythag.
func calculateDist(ax, ay, bx, by int) float64 {
	return math.Sqrt(math.Pow(float64(ax-bx), 2) + math.Pow(float64(ay-by), 2))
}

// sortByLength sorts line segments by their length, returning the shortest first
func sortByLength(in [][2]image.Point) {
	sort.Slice(in, func(a, b int) bool {
		a0, a1 := in[a][0], in[a][1]
		b0, b1 := in[b][0], in[b][1]
		return calculateDist(a0.X, a0.Y, a1.X, a1.Y) < calculateDist(b0.X, b0.Y, b1.X, b1.Y)
	})
}

// savePNG to disk
func savePNG(fpath string, in image.Image) error {
	buff := new(bytes.Buffer)
	err := png.Encode(buff, in)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fpath, buff.Bytes(), 0644)
}

// maxint returns the highest of two ints
func maxint(a, b int) int {
	if a > b {
		return a
	}
	return b
}
