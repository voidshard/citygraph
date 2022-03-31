package citygraph

import (
	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/voidshard/citygraph/internal/cell"
	"github.com/voidshard/citygraph/internal/line"
	"github.com/voidshard/citygraph/internal/voronoi"
)

var (
	// ErrCannotMeetDesiredDistricts implies we cannot satisfy the configured
	// minimum settings.
	ErrCannotMeetDesiredDistricts = fmt.Errorf("failed to place full number of desired districts")
)

// Citygraph holds our city information & handles the bulk of our math operations
type Citygraph struct {
	outline Outline

	bcfg *BuilderConfig
	cfg  *CityConfig

	rng *rand.Rand

	Districts []*District
	Walls     []*Edge           `json:",omitempty"`
	Towers    []image.Rectangle `json:",omitempty"`
	Gates     []image.Rectangle `json:",omitempty"`
	Stats     *CityStats        `json:",omitempty"`
	Seed      int64

	gb         *voronoi.Builder
	graph      *voronoi.Voronoi
	cellToDist map[int]*District
	cmap       *imageMap
}

// New creates a new Citygraph given configuaration & an Outline
func New(bcfg *BuilderConfig, cfg *CityConfig, o Outline) (*Citygraph, error) {
	cg := &Citygraph{
		bcfg:    bcfg,
		cfg:     cfg,
		outline: o,
	}
	return cg, cg.build()
}

// JSON returns the citygraph as json.
func (c *Citygraph) JSON() ([]byte, error) {
	return json.Marshal(c)
}

// SaveJSON writes a json file to the given path.
func (c *Citygraph) SaveJSON(fpath string) error {
	data, err := c.JSON()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fpath, data, 0644)
}

// Map returns the underlying CityMap.
// The map essentially holds the same data but saved graphically rather than in
// Go structs.
func (c *Citygraph) Map() CityMap {
	return c.cmap
}

// build runs the main construction logic. Order of the functions
// is important as later functions rely on things being done / not done
// to save re-processing stuff.
func (c *Citygraph) build() error {
	err := c.init()
	if err != nil {
		return err
	}

	// add user defined districts
	for _, d := range c.cfg.DistrictSites {
		id, ok := c.gb.AddSite(d.Site.X, d.Site.Y)
		if !ok {
			continue
		}

		dist := c.newDistrict(id)
		dist.Site = d.Site
		dist.Type = d.Type
		dist.HasFortifications = d.HasFortifications
		dist.HasCurtainFortifications = d.HasCurtainFortifications

		c.Stats.increment(d.Type)
	}

	// add random districts if we're under the desired number
	added, err := c.randomDistricts()
	if err != nil {
		return err
	}

	// now that we have all the districts, compute the voronoi
	c.graph = c.gb.Voronoi()

	// figure out if our randomly placed districts need shuffling around
	err = c.verifyDistrictLocations(added)
	if err != nil {
		return err
	}

	if c.cfg.Fortifications != nil {
		// if required, place city / district walls
		err = c.addWalls(added)
		if err != nil {
			return err
		}
	}

	// now that all the district locations / types are set, move on to roads
	err = c.addMainRoads()
	if err != nil {
		return err
	}

	err = c.addMinorRoads()
	if err != nil {
		return err
	}

	wallroadwidth := 0
	if c.cfg.Fortifications != nil {
		wallroadwidth = c.cfg.Fortifications.WallBorderRoadWidth
	}
	c.cmap.endDraw(wallroadwidth, c.outline) // tell the map we're done painting things

	err = c.addBuildings()

	return err
}

// addWalls adds all walls (both citywalls & district curtain walls)
func (c *Citygraph) addWalls(in []*District) error {
	// ostensibly this is quite straight forward, it's made somewhat more complicated
	// by us having to place gatehouses .. which logically need a bit of care.
	// Gatehouses for districts with curtain walls ideally open into the "inside"
	// (walled) section(s) of the city.
	// Gatehouses "inside" open to "outside" of the city.
	// However curtain walled districts are not always inside the city, so
	// if no "inside" sections are reachable, then a curtain walled district
	// might open to the outside too. Technically this makes it *not* curtain walled
	// mearly "walled" but outside of the main city walls .. but eh.

	for _, d := range in {
		dcfg, ok := c.bcfg.Districts[d.Type]
		if !ok {
			return fmt.Errorf("no config for district type %s", d.Type)
		}
		d.HasFortifications = dcfg.HasFortifications
		d.HasCurtainFortifications = dcfg.HasCurtainFortifications
	}

	curtainWall := []*District{}
	insideDists := []*District{}
	outsideDists := []*District{}

	// work out whether districts are inside, outside or "curtain" (self contained)
	for _, d := range c.Districts {
		if d.HasCurtainFortifications {
			curtainWall = append(curtainWall, d)
		} else if d.HasFortifications {
			insideDists = append(insideDists, d)
		} else {
			outsideDists = append(outsideDists, d)
			continue // we don't need to find neighbours
		}
	}

	// if required, choose some more districts to be "inside"
	needed := c.cfg.Fortifications.MinFortifiedSites - len(insideDists)
	if needed > 0 { // we need to add some more
		if needed >= len(outsideDists) {
			// kind of silly but .. eh.
			insideDists = append(insideDists, outsideDists...)
		} else {
			sortDistrictsByDistance(c.cfg.Centre, outsideDists)
			for _, d := range outsideDists[:needed] {
				insideDists = append(insideDists, d)
				d.HasFortifications = true
			}
			outsideDists = outsideDists[needed:]
		}
	}

	// work out potential locations of gates for each district
	gatesIdeal := map[int][]*gateLocation{}
	gatesOther := map[int][]*gateLocation{}
	ghWidth := (c.cfg.Fortifications.TowerArea.Max.X-c.cfg.Fortifications.TowerArea.Min.X)*2 + c.cfg.Fortifications.GatehouseArea.Max.X - c.cfg.Fortifications.GatehouseArea.Min.X
	ghHeight := (c.cfg.Fortifications.TowerArea.Max.Y-c.cfg.Fortifications.TowerArea.Min.Y)*2 + c.cfg.Fortifications.GatehouseArea.Max.Y - c.cfg.Fortifications.GatehouseArea.Min.Y

	// What we want here is all edges to valid districts that we could
	// (probably) put a gatehouse in.
	for _, d := range append(insideDists, curtainWall...) { // nb. no need for "outside"
		site := c.graph.SiteByID(d.ID)
		inside := d.HasFortifications && !d.HasCurtainFortifications

		sourceID := d.ID
		if inside {
			sourceID = -1
		}

		for _, n := range site.Neighbours() {
			isIdeal := true
			dist, _ := c.cellToDist[n.Site.ID()]
			if dist.HasCurtainFortifications {
				continue
			} else if inside {
				if dist.HasFortifications {
					continue
				}
			} else {
				isIdeal = dist.HasFortifications
			}

			gideal, ok := gatesIdeal[sourceID]
			if !ok {
				gideal = []*gateLocation{}
			}
			gother, ok := gatesOther[sourceID]
			if !ok {
				gother = []*gateLocation{}
			}

			for _, e := range n.Edges {
				length := int(calculateDist(e[0].X, e[0].Y, e[1].X, e[1].Y))
				if length < ghWidth || length < ghHeight {
					continue
				}

				gl := &gateLocation{In: site, InDist: d, Out: n.Site, OutDist: dist, Edge: e}
				if isIdeal {
					gideal = append(gideal, gl)
				} else {
					gother = append(gother, gl)
				}
			}

			gatesIdeal[sourceID] = gideal
			gatesOther[sourceID] = gother
		}
	}

	allTowers := []image.Rectangle{}
	if len(insideDists) > 0 {
		gates, _ := gatesIdeal[-1]
		ws, gs, ts := c.wallDistricts(insideDists, outsideDists, gates, allTowers)
		c.Walls = ws
		c.Towers = ts
		allTowers = append(allTowers, ts...)
		for _, gate := range gs {
			c.Gates = append(c.Gates, gate.Gatehouse)
		}
	}

	for _, d := range curtainWall {
		gates, ok := gatesIdeal[d.ID]
		if !ok || len(gates) == 0 { // probably we aren't within the city proper
			gates, _ = gatesOther[d.ID]
		}
		ws, gs, ts := c.wallDistricts([]*District{d}, nil, gates, allTowers)
		d.Walls = ws
		d.Towers = ts
		allTowers = append(allTowers, ts...)
		for _, gate := range gs {
			d.Gates = append(d.Gates, gate.Gatehouse)
		}

	}

	return nil
}

// wallDistricts builds walls / towers / gates around the given `in` district(s)
func (c *Citygraph) wallDistricts(in, out []*District, gates []*gateLocation, allTowers []image.Rectangle) ([]*Edge, map[string]*gateLocation, []image.Rectangle) {
	towers := []image.Rectangle{}
	madeGates := map[string]*gateLocation{}

	tryPlaceTower := func(p image.Point, mdbt int) {
		tower, ok := c.towerFits(p.X, p.Y)
		if !ok {
			return
		}
		if mdbt > 0 {
			for _, t := range append(allTowers, towers...) {
				tx, ty := (t.Max.X+t.Min.X)/2, (t.Max.Y+t.Min.Y)/2
				if int(calculateDist(tx, ty, p.X, p.Y)) < mdbt {
					return
				}
			}
		}
		c.cmap.drawTower(tower.Min, tower.Max.X-tower.Min.X, tower.Max.Y-tower.Min.Y)
		towers = append(towers, tower)
	}

	fillWithTowers := func(a, b image.Point) {
		mdbt := c.cfg.Fortifications.MinDistBetweenTowers

		// firstly, place at both ends
		tryPlaceTower(a, mdbt/2)
		tryPlaceTower(b, mdbt/2)

		pnts := line.PointsBetween(a, b)
		for i := mdbt; i < len(pnts); i += mdbt {
			tryPlaceTower(pnts[i], mdbt)
		}
	}

	toEdgeID := func(a, b image.Point) string {
		if b.X < a.X {
			a, b = b, a
		} else if a.X == b.X && b.Y < a.Y {
			a, b = b, a
		}
		return fmt.Sprintf("%d,%d-%d,%d", a.X, a.Y, b.X, b.Y)
	}

	width := c.cfg.Fortifications.WallWidth
	maxGates := c.cfg.Fortifications.MaxCityGates
	if len(in) == 0 {
		return []*Edge{}, madeGates, towers // ??
	} else if len(in) == 1 {
		maxGates = 1 // since we're walling a single district
		out = []*District{}
		for _, d := range c.Districts {
			if d.ID == in[0].ID {
				continue
			}
			out = append(out, d)
		}
		width = c.cfg.Fortifications.CurtainWallWidth
	}

	inside := []voronoi.Site{}
	for _, d := range in {
		inside = append(inside, c.graph.SiteByID(d.ID))
	}

	outside := []voronoi.Site{}
	for _, d := range out {
		outside = append(outside, c.graph.SiteByID(d.ID))
	}

	for _, loc := range gates {
		if len(madeGates) >= maxGates {
			break
		}
		// figure out where we'd like to put towers / gatehouse
		err := loc.determinePlacements(c.graph, c.cfg.Fortifications.TowerArea, c.cfg.Fortifications.GatehouseArea)
		if err != nil {
			continue
		}

		// decide if the towers/gatehouse fit in the map
		if !loc.fortificationsFit(c.cmap, c.outline) {
			continue
		}

		madeGates[toEdgeID(loc.Edge[0], loc.Edge[1])] = loc

		for _, wall := range loc.Walls {
			c.cmap.drawWall(wall[0], wall[1], width)
		}
		for _, tower := range loc.Towers {
			c.cmap.drawTower(tower.Min, tower.Max.X-tower.Min.X, tower.Max.Y-tower.Min.Y)
			towers = append(towers, tower)
		}
		c.cmap.drawGatehouse(loc.Gatehouse.Min, loc.Gatehouse.Max.X-loc.Gatehouse.Min.X, loc.Gatehouse.Max.Y-loc.Gatehouse.Min.Y)
	}

	edges := []*Edge{}
	wall := cell.Circut(c.cfg.Area, inside, outside)
	for _, segment := range wall {
		land, water, walls := c.lineSegments(segment[0], segment[1], nil)
		if len(land)+len(water) == 0 {
			continue
		}

		gate, hasGate := madeGates[toEdgeID(segment[0], segment[1])]
		e := &Edge{Path: segment, Sections: []*Section{}}

		for _, path := range append(land, walls...) {
			if hasGate && (gate.withinGateCourtyard(path[0]) || gate.withinGateCourtyard(path[1])) {
				// we've already drawn the walls, we just need to add the edges
				// to our metadata
				for _, gatehouseWall := range gate.Walls {
					e.Sections = append(e.Sections, &Section{Path: gatehouseWall})
				}
				continue
			}
			c.cmap.drawWall(path[0], path[1], width)
			e.Sections = append(e.Sections, &Section{Path: path})

			fillWithTowers(path[0], path[1])
		}
		for _, path := range water {
			dist := int(calculateDist(path[0].X, path[0].Y, path[1].X, path[1].Y))
			if c.cfg.Fortifications.MaxBridgeWallLength > 0 && dist > c.cfg.Fortifications.MaxBridgeWallLength {
				continue
			}
			c.cmap.drawWall(path[0], path[1], width)
			e.Sections = append(e.Sections, &Section{Path: path, Bridge: true})

			fillWithTowers(path[0], path[1])
		}

		edges = append(edges, e)
	}

	return edges, madeGates, towers
}

// towerFits is a somewhat unique building Fit func that
// - allows a tower to sit atop a wall (makes sense)
// - allows a tower to be built in "bridgeable" pixels (ie, in water)
func (c *Citygraph) towerFits(px, py int) (image.Rectangle, bool) {
	// px,py is assumed to be the centre of the tower
	twr := c.cfg.Fortifications.TowerArea
	tw := twr.Max.X - twr.Min.X
	th := twr.Max.Y - twr.Min.Y
	area := image.Rect(px-tw/2, py-th/2, px+tw/2, py+th/2)
	for x := area.Min.X; x < area.Max.X; x++ {
		for y := area.Min.Y; y < area.Max.Y; y++ {
			// nb. we allow building over walls & in water ("bridgeable")
			if c.cmap.IsTower(x, y) || c.cmap.IsGatehouse(x, y) {
				return area, false
			}
			if c.outline.CanBuildOn(x, y) || c.outline.CanBridgeOver(x, y) {
				continue
			}
			return area, false
		}
	}
	return area, true
}

// addBuildings fills all our districts with buildings .. if they have buildings
func (c *Citygraph) addBuildings() error {
	for _, d := range c.Districts {
		site := c.graph.SiteByID(d.ID)
		if site == nil { // this .. really shouldn't happen ever
			return fmt.Errorf("site not found for id %d", d.ID)
		}

		dcfg, ok := c.bcfg.Districts[d.Type]
		if !ok {
			return fmt.Errorf("no config for district type %s", d.Type)
		}

		err := c.addBuildingsToDistrict(d, dcfg, site)
		if err != nil {
			return err
		}
	}

	return nil
}

// addBuildingsToDistrict both sets Districts in our CityMap and adds buildings.
// We do both at once because it's really the only time we iterate every pixel within
// a district .. which is absurdly slow but needs to be done.
func (c *Citygraph) addBuildingsToDistrict(d *District, dcfg *DistrictConfig, site voronoi.Site) error {
	bnds := site.Bounds()

	distbuild := newDistrictBuilder(c.cfg.Seed, c.outline, d, dcfg, site, c.cmap)

	fromCentre := bnds.Max.X + bnds.Max.Y
	var centralCoord *image.Point

	// set District in CityMap and figure out location of "central" building if set
	// TODO: figure out a sane way to merge this into the loops below
	for y := bnds.Min.Y; y < bnds.Max.Y; y++ {
		for x := bnds.Min.X; x < bnds.Max.X; x++ {
			if c.graph.SiteFor(x, y).ID() != d.ID {
				continue
			}

			err := c.cmap.setDistrict(x, y, d.Type, d.ID)
			if err != nil {
				return err
			}

			if dcfg.Central == nil {
				continue
			}

			dist := int(calculateDist(site.X(), site.Y(), x, y))
			if dist > fromCentre {
				continue
			}

			if distbuild.buildingFits(x, y, dcfg.Central) {
				centralCoord = &image.Point{X: x, Y: y}
				fromCentre = dist
			}
		}
	}
	if dcfg.Central != nil && centralCoord != nil {
		cbuild := d.addBuilding(centralCoord.X, centralCoord.Y, dcfg.Central)
		d.Central = cbuild
		c.cmap.setBuilding(centralCoord.X, centralCoord.Y, dcfg.Central)
	}

	if dcfg.Buildings == nil || len(dcfg.Buildings) == 0 {
		return nil
	}

	// to make things look a little bit more natural we'll place buildings via running around
	// the outside of the region in rings, spiralling inwards towards the centre.
	// This makes buildings appear to hug the edges / roads a lot more
	dx := bnds.Max.X - bnds.Min.X
	dy := bnds.Max.Y - bnds.Min.Y
	ilimit := dx
	if dy < dx {
		ilimit = dy
	}
	for i := 0; i < ilimit/2; i++ { // spiral inward by i going along four edges
		for x := bnds.Min.X + i; x < bnds.Max.X-i; x++ {
			if c.rng.Float64() < dcfg.BuildingDensity {
				b := distbuild.chooseBuilding(x, bnds.Min.Y+i)
				c.cmap.setBuilding(x, bnds.Min.Y+i, b)
				d.addBuilding(x, bnds.Min.Y+i, b)
			}
			if c.rng.Float64() < dcfg.BuildingDensity {
				b := distbuild.chooseBuilding(x, bnds.Max.Y-i)
				c.cmap.setBuilding(x, bnds.Max.Y-1-i, b)
				d.addBuilding(x, bnds.Max.Y-1-i, b)
			}
		}

		for y := bnds.Min.Y + i; y < bnds.Max.Y-i; y++ {
			if c.rng.Float64() < dcfg.BuildingDensity {
				b := distbuild.chooseBuilding(bnds.Min.X+i, y)
				c.cmap.setBuilding(bnds.Min.X+i, y, b)
				d.addBuilding(bnds.Min.X+i, y, b)
			}
			if c.rng.Float64() < dcfg.BuildingDensity {
				b := distbuild.chooseBuilding(bnds.Max.X-i, y)
				c.cmap.setBuilding(bnds.Max.X-1-i, y, b)
				d.addBuilding(bnds.Max.X-1-i, y, b)
			}
		}

	}

	return nil
}

// addMinorRoads builds a smaller road network within a district.
func (c *Citygraph) addMinorRoads() error {
	// for each distrct
	//  - choose random points within district bounds
	//  - make sub voronoi
	//  - for each edge draw streets
	for _, d := range c.Districts {
		dcfg, ok := c.bcfg.Districts[d.Type]
		if !ok {
			return fmt.Errorf("district config not found for type %s", d.Type)
		}
		if !dcfg.needsRoads() {
			continue
		}

		site := c.graph.SiteByID(d.ID)
		if site == nil {
			return fmt.Errorf("site not found for id %d", d.ID)
		}

		// determine block size
		// (so our sub voronoi cells can (probably) fit this)
		largestBuildingDimension := c.cfg.MinBlockSize
		if dcfg.Buildings != nil {
			for _, b := range dcfg.Buildings {
				bx := b.Area.Max.X - b.Area.Min.X
				by := b.Area.Max.Y - b.Area.Min.Y
				if bx > largestBuildingDimension {
					largestBuildingDimension = bx
				}
				if by > largestBuildingDimension {
					largestBuildingDimension = by
				}
			}
		}

		// build sub voronoi constrained inside our district
		orig := site.Bounds()

		vb := voronoi.NewBuilder(orig)
		vb.SetSeed(c.cfg.Seed/2 + int64(d.ID))

		vb.SetCandidateFilters(
			func(dx, dy int) bool {
				return c.outline.CanBuildOn(dx, dy) && site.Contains(dx, dy)
			},
		)
		vb.SetSiteFilters(
			vb.MinDistance(float64(largestBuildingDimension / 2)),
		)

		newSites := int(float64(orig.Max.X-orig.Min.X) * 0.25 * dcfg.RoadDensity)
		for i := 0; i < newSites; i++ {
			vb.AddRandomSite()
		}
		if vb.SiteCount() == 0 {
			continue // we can't break this district up any further
		}

		maxBridges := dcfg.MaxBridges

		// annnd finally, draw the roads
		vv := vb.Voronoi()
		for _, block := range vv.Sites() {
			for _, edge := range block.Edges() {
				roads, bridges, _ := c.lineSegments(edge[0], edge[1], site)
				if len(roads)+len(bridges) == 0 {
					continue
				}

				e := &Edge{Path: edge, Sections: []*Section{}}

				for _, path := range roads {
					if path[0].Y == path[1].Y || path[0].X == path[1].X {
						continue
					}
					if dcfg.RoadWidth < 2 {
						dcfg.RoadWidth = 2
					}
					c.cmap.drawRoad(path[0], path[1], dcfg.RoadWidth/2)
					e.Sections = append(e.Sections, &Section{Path: path})
				}

				sortByLength(bridges) // make shortest bridges first
				for _, path := range bridges {
					if maxBridges >= 0 && d.Stats.Bridges >= maxBridges {
						break
					}
					blen := int(calculateDist(path[0].X, path[0].Y, path[1].X, path[1].Y))
					if c.cfg.MaxBridgeLength > 0 && blen > c.cfg.MaxBridgeLength {
						continue
					}
					if c.cfg.MinBridgeLength > blen {
						continue
					}

					c.cmap.drawBridge(path[0], path[1], dcfg.RoadWidth/2)
					d.Stats.Bridges++
					e.Sections = append(e.Sections, &Section{Path: path, Bridge: true})
				}

				d.Roads = append(d.Roads, e)
			}
		}
	}

	return nil
}

// lineSegments breaks up points between (start, end) into sections of road(s) and/or bridge(s).
// If a site is given, we check that all points are within the site.
// We return the segment broken into shorter segments where all pixels share some type.
// - roads: where the pixels are buildable & nothing yet exists
// - bridges: where the pixels are bridgeable
// - walls: where a wall exists already
func (c *Citygraph) lineSegments(start, end image.Point, site voronoi.Site) ([][2]image.Point, [][2]image.Point, [][2]image.Point) {
	roads := [][2]image.Point{}
	bridges := [][2]image.Point{}
	walls := [][2]image.Point{}
	path := line.PointsBetween(start, end)

	prevIndex := -1
	prevEnum := -1

	enumNothing := 0
	enumRoad := 1
	enumBridge := 2
	enumWall := 3

	for i := range path {
		p := path[i]

		buildingID, _ := c.cmap.BuildingID(p.X, p.Y)

		me := enumNothing
		if site != nil && !site.Contains(p.X, p.Y) {
			// we don't draw outside of the site (if given)
		} else if buildingID != 0 {
			// no putting roads through building(s)
		} else if c.cmap.isFortification(p.X, p.Y) {
			me = enumWall
		} else if c.outline.CanBuildOn(p.X, p.Y) {
			me = enumRoad
		} else if c.outline.CanBridgeOver(p.X, p.Y) {
			me = enumBridge
		}

		if prevIndex == -1 {
			prevIndex = i
			prevEnum = me
			continue
		}
		if prevEnum == me {
			continue
		}

		if prevEnum == enumBridge {
			if len(roads) > 0 { // we won't start a road with a bridge
				bridges = append(bridges, [2]image.Point{path[prevIndex], path[i-1]})
			}
		} else if prevEnum == enumRoad {
			roads = append(roads, [2]image.Point{path[prevIndex], path[i-1]})
		} else if prevEnum == enumWall {
			walls = append(walls, [2]image.Point{path[prevIndex], path[i-1]})
		}

		prevIndex = i
		prevEnum = me
	}

	if prevIndex != len(path)-1 {
		if prevEnum == enumBridge {
			bridges = append(bridges, [2]image.Point{path[prevIndex], path[len(path)-1]})
		} else if prevEnum == enumRoad {
			roads = append(roads, [2]image.Point{path[prevIndex], path[len(path)-1]})
		} else if prevEnum == enumWall {
			walls = append(walls, [2]image.Point{path[prevIndex], path[len(path)-1]})
		}
	}

	return roads, bridges, walls
}

// addMainRoads draws the main district roads in
func (c *Citygraph) addMainRoads() error {
	minX := c.cfg.Area.Min.X
	minY := c.cfg.Area.Min.Y
	maxX := c.cfg.Area.Max.X
	maxY := c.cfg.Area.Max.Y

	width := c.cfg.MainRoadWidth
	createdBridges := 0
	maxBridges := c.cfg.MaxBridges

	for _, d := range c.Districts {
		dcfg, ok := c.bcfg.Districts[d.Type]
		if !ok {
			return fmt.Errorf("district config not found for type %s", d.Type)
		}
		if !dcfg.needsRoads() {
			continue
		}

		s := c.graph.SiteByID(d.ID)
		if s == nil {
			return fmt.Errorf("site not found for id %d", d.ID)
		}

		// in theory we'd just whack down a road, but we need to check
		// that we can actually build on the squares the road would cross
		// and/or consider bridges
		for _, edge := range s.Edges() {
			if (edge[0].X == minX || edge[0].Y == minY) && (edge[1].X == minX || edge[1].Y == minY) {
				continue
			}
			if (edge[0].X == maxX || edge[0].Y == maxY) && (edge[1].X == maxX || edge[1].Y == maxY) {
				continue
			}

			roads, bridges, _ := c.lineSegments(edge[0], edge[1], nil)
			if len(roads)+len(bridges) == 0 {
				continue
			}

			e := &Edge{Path: edge, Sections: []*Section{}}

			for _, path := range roads {
				c.cmap.drawRoad(path[0], path[1], width/2)
				e.Sections = append(e.Sections, &Section{Path: path})
			}

			sortByLength(bridges) // make shortest first (seems logical)

			for _, path := range bridges {
				if maxBridges >= 0 && createdBridges >= maxBridges {
					break
				}
				blen := int(calculateDist(path[0].X, path[0].Y, path[1].X, path[1].Y))
				if c.cfg.MaxBridgeLength > 0 && blen > c.cfg.MaxBridgeLength {
					continue
				}
				if c.cfg.MinBridgeLength > blen {
					continue
				}
				c.cmap.drawBridge(path[0], path[1], width/2)
				createdBridges++
				e.Sections = append(e.Sections, &Section{Path: path, Bridge: true})
			}

			d.Roads = append(d.Roads, e)
		}
	}
	return nil
}

// verifyDistrictLocations - we do our best in randomDistricts() to pick sensible locations
// but we have to have chosen all sites in order for us to count their Buildable/DockSuitable
// co-ords. Because of this we have to run through them all & do some last minute
// adjustments now that the district borders are set.
// Ie.
//  - ensure Docks include the right number of SuitableDock squares
//  - ensure Non Empty, Non Dock districts include the min number of Buildable squares
func (c *Citygraph) verifyDistrictLocations(in []*District) error {
	// in order to keep this .. slightly more efficient .. I'm rolling together
	// related functions so we can cut out as much work as possible

	// problem children
	docks := []*District{}

	// potential solutions
	potentialdock := []*District{}

	// step 0; collect stats
	for _, d := range in {
		site := c.graph.SiteByID(d.ID)
		if site == nil { // this .. really shouldn't happen ever
			return fmt.Errorf("site not found for id %d", d.ID)
		}

		gen := site.AllContains()
		for {
			p := gen.Next()
			if p == nil {
				break
			}

			if c.outline.CanBuildOn(p.X, p.Y) {
				d.Stats.Buildable++
			}
			if c.outline.CanBridgeOver(p.X, p.Y) {
				d.Stats.Bridgeable++
			}
			if c.outline.SuitableDock(p.X, p.Y) {
				d.Stats.DockSuitable++
			}
		}

		if d.Type == Docks && d.Stats.DockSuitable < c.cfg.MinDockSize {
			docks = append(docks, d) // docks we have to move
		} else if d.Type != Docks && d.Stats.DockSuitable >= c.cfg.MinDockSize {
			potentialdock = append(potentialdock, d) // districts we could make docks
		}
	}

	if len(docks) == 0 {
		return nil
	}

	probDistTotal := 0.0
	for _, typ := range allDistricts {
		dc, ok := c.bcfg.Districts[typ]
		if !ok {
			continue
		}
		probDistTotal += dc.Probability
	}

	// ok time for some shuffling
	for { // step 1; fix our dock situation
		if len(docks) == 0 {
			break
		}

		d := docks[len(docks)-1]
		docks = docks[:len(docks)-1]

		// idea #1 switch dock wannabe with 'potential dock'
		if len(potentialdock) > 0 {
			last := potentialdock[len(potentialdock)-1]

			lt := last.Type
			last.Type = Docks
			d.Type = lt

			potentialdock = potentialdock[:len(potentialdock)-1]
			continue
		}

		// idea #2 no swaps available so let's make this not a dock
		minInCity := 0
		dcfg, ok := c.bcfg.Districts[Docks]
		if ok {
			minInCity = dcfg.MinInCity
		}

		if c.Stats.count(Docks) > minInCity {
			// ok, we're allowed to remove a dock, let's choose another type ..
			for {
				t := c.chooseDistrictType(probDistTotal)
				if t == Docks {
					continue // we specifically want to avoid a dock
				}
				dc, ok := c.bcfg.Districts[t]
				if !ok {
					continue // shouldn't happen
				}
				if dc.MaxInCity > 0 && c.Stats.count(t) >= dc.MaxInCity {
					continue // too many of this kind of district
				}
				d.Type = t
				c.Stats.increment(t)
				c.Stats.decrement(Docks)
				break
			}

			continue
		}

		// we can't remove the dock & we can't move it .. ouch
		return fmt.Errorf("unable to find place for docks district")
	}

	return nil
}

// randomDistricts adds more districts not specifically set down by a user
// but where they request "some number of districts" ie 'DesiredDistricts'
func (c *Citygraph) randomDistricts() ([]*District, error) {
	toSet := []*District{}

	if len(c.Districts) >= c.cfg.DesiredDistricts {
		return toSet, nil // we have enough sites already
	}

	// since we don't have enough - decide more district types to add.
	// The approach is simple; we'll first decide the districts we want by type
	// then we'll worry about placing them somewhere.
	// Naturally the first districts we'll add are those to meet our
	// district 'MinInCity' config (assuming they're not added above)
	tempStats := map[DistrictType]int{}
	dtypes := []DistrictType{}
	probDistTotal := 0.0 // for normalisation
	for _, typ := range allDistricts {
		dc, ok := c.bcfg.Districts[typ]
		if !ok {
			continue
		}

		probDistTotal += dc.Probability

		// add districts so we have the min number desired
		for i := c.Stats.count(typ); i < dc.MinInCity; i++ {
			dtypes = append(dtypes, typ)
			unsetCount, _ := tempStats[typ]
			tempStats[typ] = unsetCount + 1
		}
	}

	// add random sites - we'll make more attempts than needed incase
	// we randomly pick some invalid points
	c.gb.SetCandidateFilters(
		func(x, y int) bool {
			// basically we want to check that area around district centre
			// has enough usable land
			landcount := 0
			for dx := x - c.cfg.MinDistrictSize/2; dx < x+c.cfg.MinDistrictSize/2; dx++ {
				for dy := y - c.cfg.MinDistrictSize/2; dy < y+c.cfg.MinDistrictSize/2; dy++ {
					if c.outline.CanBuildOn(dx, dy) {
						landcount++
					}
					if landcount >= c.cfg.MinDistrictSize {
						return true
					}
				}
			}
			return landcount >= c.cfg.MinDistrictSize
		},
	)
	c.gb.SetSiteFilters(
		c.gb.MinDistance(float64(c.cfg.MinDistrictSize / 2)),
	)
	for i := 0; i < c.cfg.DesiredDistricts*5; i++ {
		if len(toSet)+len(c.Districts) >= c.cfg.DesiredDistricts {
			break // we have enough
		}

		x, y, id, ok := c.gb.AddRandomSite()
		if !ok {
			continue
		}

		dist := c.newDistrict(id)
		dist.Site = image.Pt(x, y)
		toSet = append(toSet, dist)
	}
	if len(toSet) < len(dtypes) { // note we're only checking if we can't fit the min districts
		return nil, fmt.Errorf("%w cant fit %d of %d", ErrCannotMeetDesiredDistricts, len(toSet), len(dtypes))
	}

	// now that we have our "min number of districts" we'll keep adding
	// district types at random until we have the desired number
	for i := len(dtypes); i < len(toSet); i++ {
		for {
			t := c.chooseDistrictType(probDistTotal)
			dc, ok := c.bcfg.Districts[t]
			if !ok {
				continue
			}
			unsetCount, _ := tempStats[t]
			if dc.MaxInCity > 0 && c.Stats.count(t)+unsetCount >= dc.MaxInCity {
				continue // too many of this kind of district
			}
			dtypes = append(dtypes, t)
			tempStats[t] = unsetCount + 1
			break
		}
	}

	// and now we can choose which district type goes to which site
	sortDistrictsByDistance(c.cfg.Centre, toSet)
	sortTypesByDesirability(dtypes)
	for i, d := range toSet { // zip the two lists together
		d.Type = dtypes[i]
		c.Stats.increment(d.Type)
	}

	return toSet, nil
}

// chooseDistrictType returns a district type at random, taking into account
// min / maxes, already existing districts & probabilities
func (c *Citygraph) chooseDistrictType(total float64) DistrictType {
	rv := c.rng.Float64()
	sofar := 0.0

	for _, typ := range allDistricts {
		dist, ok := c.bcfg.Districts[typ]
		if !ok {
			continue
		}
		if dist.Probability <= 0 {
			continue
		}

		prob := dist.Probability / total // normalised
		if rv <= prob+sofar {
			return typ
		}
		sofar += prob
	}

	return Empty
}

// newDistrict builds a new district struct & wires it in.
// Nb. we don't set district type here so don't increment city stats
func (c *Citygraph) newDistrict(id int) *District {
	dist := &District{
		ID:        id,
		Buildings: []*Building{},
		Stats: &DistrictStats{
			BuildingsByID: map[int]int{},
		},
		Roads:  []*Edge{},
		Walls:  []*Edge{},
		Towers: []image.Rectangle{},
		Gates:  []image.Rectangle{},
	}
	c.cellToDist[id] = dist
	c.Districts = append(c.Districts, dist)
	return dist
}

// init sets up the city graph with a bunch of stuff. Probably could be rolled into new.
func (c *Citygraph) init() error {
	if c.cfg.Seed == 0 {
		c.cfg.Seed = time.Now().UnixNano()
	}
	c.Seed = c.cfg.Seed
	c.rng = rand.New(rand.NewSource(c.cfg.Seed))
	c.cmap = newMap(c.cfg.Area)

	if c.cfg.MainRoadWidth < 1 {
		c.cfg.MainRoadWidth = 1
	}

	if c.cfg.Centre == image.ZP {
		c.cfg.Centre = image.Pt( // middle of area
			(c.cfg.Area.Max.X-c.cfg.Area.Min.X)/2,
			(c.cfg.Area.Max.Y-c.cfg.Area.Min.Y)/2,
		)
	}

	c.cellToDist = map[int]*District{}

	c.Stats = newCityStats()
	c.Districts = []*District{}
	c.Walls = []*Edge{}
	c.Gates = []image.Rectangle{}
	c.Towers = []image.Rectangle{}

	c.gb = voronoi.NewBuilder(c.cfg.Area)
	c.gb.SetSeed(c.cfg.Seed)

	return nil
}
