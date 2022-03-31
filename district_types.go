package citygraph

import (
	"image"
	"sort"
)

// DistrictType indicates roughly what one might find in a given district.
// In practice a city wont simply contain temples in the Temple district
// or whatever, but .. still .. gives us a somewhat generic way to think
// of the city in districts with rough purposes.
type DistrictType string

const (
	Park              = "park"                    // greenery, trees, grass
	Temple            = "temple"                  // major temples, shrines, festival squares
	Civic             = "civic"                   // city hall(s), courts
	Graveyard         = "graveyard"               // for people after best-by date
	ResidentialUpper  = "residential-upperclass"  // large mansions, stately homes, fancy shops
	ResidentialMiddle = "residential-middleclass" // nice homes of those living comfortably, taverns, shops
	ResidentialLower  = "residential-lowerclass"  // smaller homes, taverns and inns of more dubious repute
	ResidentialSlum   = "residential-slum"        // mixture of homes, lean-tos, tents
	Abandoned         = "abandoned"               // an area of the city mostly destroyed, abandoned
	Fortress          = "fortress"                // castle, fort, possibly fighting arenas
	Market            = "market"                  // market probably popup shops, livestock, produce sales
	Commercial        = "commercial"              // shops of all sorts
	Square            = "square"                  // fountain(s), statues & generally an area to gather
	Industrial        = "industrial"              // industrial parts of town; smelters, tanneries, workshops
	Warehouse         = "warehouse"               // lots large buildings, possibly behind fences
	Barracks          = "barracks"                // barracks, training yards, target ranges
	Prison            = "prison"                  // large buildings, fenced yards, walls might not go a miss
	Docks             = "docks"                   // where boats, rafts, skyships .. etc make port
	Research          = "research"                // (probably wealthy) district for learning, universities, schools
	Fields            = "fields"                  // space for farmland, crops, homesteads
	Empty             = "empty"                   // nothing
)

var (
	allDistricts = []DistrictType{
		// all districts ordered by their relative closeness to the centre
		Fortress, Civic, ResidentialUpper, Temple, Square, Market, Commercial, Park,
		ResidentialMiddle, Research, Graveyard,
		ResidentialLower, Docks, ResidentialSlum, Industrial, Warehouse, Barracks, Prison,
		Fields, Abandoned, Empty,
	}

	districtindex = map[DistrictType]int{
		Empty:             0,
		Fortress:          1,
		Civic:             2,
		ResidentialUpper:  3,
		Temple:            4,
		Square:            5,
		Market:            6,
		Commercial:        7,
		Park:              8,
		ResidentialMiddle: 9,
		Research:          10,
		Graveyard:         11,
		ResidentialLower:  12,
		Docks:             13,
		ResidentialSlum:   14,
		Industrial:        15,
		Warehouse:         16,
		Barracks:          17,
		Prison:            18,
		Fields:            19,
		Abandoned:         20,
	}

	invDistrictIndex = map[int]DistrictType{}
)

func init() {
	for k, v := range districtindex {
		invDistrictIndex[v] = k
	}
}

// ID returns the index of a district type
func (d DistrictType) ID() int {
	v, ok := districtindex[d]
	if !ok {
		return 0
	}
	return v
}

// districtForID is the inversion of DistrictType.ID()
func districtForID(i int) DistrictType {
	dtype, ok := invDistrictIndex[i]
	if !ok {
		return Empty
	}
	return dtype
}

// desirability allows us to order districts by how fancy / close to the centre
// we might expect them to be.
// tl;dr civic, temple, upperclass, parks sit close to the centre.
// farms, cheap housing, industrial sit further out.
func (d DistrictType) desirability() int {
	id := d.ID()
	if id == 0 { // empty
		return len(allDistricts) + 1
	}
	return id
}

// AllDistrictTypes returns all known DistrictType enums
func AllDistrictTypes() []DistrictType {
	return allDistricts
}

// sortTypesByDesirability puts the more desireable / wealthy districts first
func sortTypesByDesirability(in []DistrictType) {
	sort.Slice(in, func(a, b int) bool {
		da := in[a].desirability()
		db := in[b].desirability()
		return da < db
	})
}

// sortDistrictsByDistance sorts districts by how close they are to some point p
func sortDistrictsByDistance(p image.Point, in []*District) {
	sort.Slice(in, func(a, b int) bool {
		dista := calculateDist(in[a].Site.X, in[a].Site.Y, p.X, p.Y)
		distb := calculateDist(in[b].Site.X, in[b].Site.Y, p.X, p.Y)
		return dista < distb
	})
}
