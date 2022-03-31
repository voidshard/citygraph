package citygraph

import (
	"image"
)

// BuilderConfig outlines settings that are hopefully relevant to
// multiple cities - laying out generally probabilities of districts
// appearing, min / max counts, building sizes, whether certain districts
// should be walled or not etc.
type BuilderConfig struct {
	Districts map[DistrictType]*DistrictConfig
}

// DistrictConfig outlines general information for a district of a given type.
type DistrictConfig struct {
	MaxInCity                int // ignored if 0, "Empty" districts may violate this
	MinInCity                int
	Probability              float64
	Buildings                []*BuildingConfig
	Central                  *BuildingConfig // a building that should be (if possible) centre of the district
	RoadWidth                int             // width of roads within district
	RoadDensity              float64         // higher values will create more roads
	MaxBridges               int             // bridges allowed inside the district
	BuildingDensity          float64         // where 1 is "place a building where-ever possible" and 0 is "place nothing"
	HasFortifications        bool            // true if the district is surrounded by city wall / towers / gatehouses
	HasCurtainFortifications bool            // true if the district has it's own wall / towers / gatehouse
}

// needsRoads returns if the district is configured to have roads (at all).
// If not we can save on some maths
func (d *DistrictConfig) needsRoads() bool {
	return d.RoadWidth > 0 && d.RoadDensity > 0
}

// BuildingConfig outlines generic information for a building.
// - id
// - how large the footprint is
// - min / max in a district
// - how likely it is to occur randomly
// Note that this building might be any height or depth .. citygraph only
// worries about the land area the building consumes. A building in this
// sense might be surrounded by a fence, have a garden, be a skyscraper,
// a plaza, have it's own internal courtyard or whatever.
type BuildingConfig struct {
	ID            int // IDs are returned by citymap's BuildingID() and should be non-zero
	Area          image.Rectangle
	MaxInCity     int // ignored if 0
	MaxInDistrict int // ignored if 0
	MinInDistrict int
	Probability   float64
}

// CityConfig hold configuaration for a given city.
// Many settings are not *strictly* required but produce very strange results
// if not given. It's probably safter to set most ..
type CityConfig struct {
	// Area constrains the bounds of the city, required
	Area image.Rectangle

	// Width of the main road(s) between districts.
	// Should be divisable by 2.
	MainRoadWidth int

	// Max number of bridges (main roads between districts).
	// Does *not* apply to bridges within districts (if MaxBridges
	// is set in DistrictConfig(s))
	MaxBridges int // less than 0 implies "no max"

	// MaxBridgeLength
	// Applies to all (road) bridges over "bridgeable" tiles
	// 0 or less is "no max"
	MaxBridgeLength int

	// MinBridgeLength
	// Recommended to be set to the min river width to curb of
	// conditions where bridges can half span & turn in the middle
	// of waterways. Technically possible I guess, but certainly odd.
	MinBridgeLength int

	// Specify centres of districts.
	// These are always placed as given. They count towards the district
	// count for the purpose of min/max districts per city.
	DistrictSites []*DistrictSite

	// DesiredDistricts number of districts (including DistrictSites)
	// If DesiredDistricts < DistrictSites random sites will be selected
	// for new districts attempting to ensure MinDistrictSize
	// Nb. this is a best-effort setting we do *not* guarantee this
	// many districts (depends if we can fit them all in!)
	DesiredDistricts int

	// Min size of districts (in terms of area).
	// We attempt to ensure that non Empty districts include at least
	// this many squares of "Buildable" land
	MinDistrictSize int

	// Min size of blocks (sub regions of districts), works similarly
	// to MinDistrictSize otherwise.
	// In practical use we will use the largest X or Y of the largest
	// building in this districts configured building(s) -- in the hopes
	// that then even the largest building can actually fit in a block
	MinBlockSize int

	// Min number of (x,y) co-ords in a district that are required to
	// be "SuitableDock" (see interface.go) for a district to be
	// eligable as "Docks" (DistrictType)
	MinDockSize int

	// Seed for rng (random number chosen if not set)
	Seed int64

	// "Centre" of city ("wealthier" districts are placed closer to this).
	// Centre of the given Area chosen if not given
	Centre image.Point

	// Fortifications allows one to configure city walls, towers, gatehouses etc.
	// We also have overrides that apply to "walled" part of the city.
	// Optional. If not given no walls / towers / gates will be plaed.
	Fortifications *FortificationSettings
}

// FortificationSettings configures information specific to walls / towers / gates.
type FortificationSettings struct {
	// Set how many city gates there can be.
	MaxCityGates int

	// Applies to section of wall (fortifications) that wish to cross
	// "bridgeable" tiles.
	// Ie. a wall that goes over a river might be fine, over the sea
	// might be strange.
	// 0 or less is "no max"
	MaxBridgeWallLength int

	// Thickness of walls of districts with curtain walls surrounding them
	CurtainWallWidth int

	// Thickness of main city wall(s)
	WallWidth int

	// Min dist between towers along the wall.
	// This restriction is relaxed in a few places
	// - ajoining pieces of walls
	// - around gatehouses
	MinDistBetweenTowers int

	// Dimensions of a tower placed along wall(s)
	TowerArea image.Rectangle

	// Dimensions of a gatehouse placed in walls to allow road(s)
	GatehouseArea image.Rectangle

	// MinFortifiedSites ensures that the given number of districts are walled,
	// selecting those of highest "value" / closest to the city centre (if for
	// example enough district sites with "HasFortifications" or districts with
	// "HasFortifications" in their config exist).
	// That is, assuming
	// - you gave a Temple district in DistrictSites with "HasFortifications"
	// - you specified that Civic and Fortress districts "HasFortifications"
	// - we randomly created 2 civic & 2 fortress districts
	// - we'd then have 5 "FortifiedSites"
	// - if the MinFortifiedSites is *higher* than 5 we would randomly select
	//   more sites to be fortified
	// Note that you could specify a DistrictSite with "HasFortifications"
	// even when (in the DistrictConfig for that DistrictType) it would
	// not generally have fortifications.
	//
	// HasCurtainFortifications are district specific & each district
	// either has it's own dedicated wall or does not.
	MinFortifiedSites int

	// The width of road(s) that run alongside wall(s).
	WallBorderRoadWidth int
}

// DistrictSite allows one to specify where a specific district by type sits
// within the city area. Intended to be used when you *know* where you want specific things to
// be exactly & want Citygraph to fill it in / add more random districts around it etc.
// Good examples of this might be
// - a Docks district for a city that must have a port at a specific spot (ie. mouth of river)
// - a Fortress district(s) for a city that has a castle / fort / similar at a given location
// - a Temple district on a designated holy site
// - etc
type DistrictSite struct {
	Type                     DistrictType // defined in district_types.go
	Site                     image.Point  // district centre (approx for voronoi)
	HasFortifications        bool         // true if the district is surrounded by city wall / towers / gatehouses
	HasCurtainFortifications bool         // true if the district has it's own wall / towers / gatehouse
}
