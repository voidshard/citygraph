package citygraph

import (
	"image"
)

// CityStats holds generic stats about the city
type CityStats struct {
	// Count of the number of districts of a given type
	DistrictsByType map[DistrictType]int
}

// newCityStats returns blank CityStats
func newCityStats() *CityStats {
	return &CityStats{DistrictsByType: map[DistrictType]int{}}
}

// increment DistrictsByType by 1
func (c *CityStats) increment(t DistrictType) {
	count, _ := c.DistrictsByType[t]
	c.DistrictsByType[t] = count + 1
}

// decrement DistrictsByType by 1
func (c *CityStats) decrement(t DistrictType) {
	count, _ := c.DistrictsByType[t]
	c.DistrictsByType[t] = count - 1
}

// count returns number of districts by type
func (c *CityStats) count(t DistrictType) int {
	count, _ := c.DistrictsByType[t]
	return count
}

// District represents a section of the city.
type District struct {
	// ID for this district
	ID int

	// Type see district_types.go
	Type DistrictType

	// Centre of district (voronoi site)
	Site image.Point

	// buildings in this district
	Buildings []*Building `json:",omitempty"`
	Central   *Building   `json:",omitempty"`

	// general stats about the district
	Stats *DistrictStats `json:",omitempty"`

	// if the district has walls
	HasFortifications        bool `json:",omitempty"`
	HasCurtainFortifications bool `json:",omitempty"`

	// information about roads, walls, towers, gates
	Roads  []*Edge           `json:",omitempty"`
	Walls  []*Edge           `json:",omitempty"`
	Towers []image.Rectangle `json:",omitempty"`
	Gates  []image.Rectangle `json:",omitempty"`
}

// Edge represents a complete line along Path that is broken into
// Sections, which are each parts of the Path.
// Ie. an edge from a - z might have three parts
// - a->f: a stretch of road
// - g->m: a bridge
// - n->z: a stretch of road
// that together make up what we would call (in this case) a
// single "road"
type Edge struct {
	Path     [2]image.Point
	Sections []*Section
}

// Section is a piece of an edge
type Section struct {
	Path   [2]image.Point
	Bridge bool `json:",omitempty"`
}

// Building represents an area of land for a given purpose / structure.
// The ID here is from the matching BuildingConfig.ID & the Area
// is the same size (naturally the Area.Min here is unique & tells us
// the top-left corner of the building location)
type Building struct {
	ID   int
	Area image.Rectangle
}

// DistrictStats holds generic stats about the district
type DistrictStats struct {
	// numbers of pixels of each type (see interface.go)
	DockSuitable int `json:",omitempty"`
	Buildable    int `json:",omitempty"`
	Bridgeable   int `json:",omitempty"`

	// counts of interesting features, buildings by ID (see BuildingConfig)
	BuildingsByID map[int]int `json:",omitempty"`
	Bridges       int         `json:",omitempty"`
}

// addBuilding b at the given x,y
func (d *District) addBuilding(x, y int, b *BuildingConfig) *Building {
	if b == nil {
		return nil
	}

	count, _ := d.Stats.BuildingsByID[b.ID]
	d.Stats.BuildingsByID[b.ID] = count + 1

	build := &Building{ID: b.ID, Area: image.Rect(x, y, b.Area.Max.X-b.Area.Min.X, b.Area.Max.Y-b.Area.Min.Y)}
	d.Buildings = append(d.Buildings, build)

	return build
}
