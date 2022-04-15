package main

import (
	"fmt"
	"image"

	"github.com/voidshard/citygraph"
)

// testOutline acts as our Outline interface allowing citygraph to understand
// a bit about the underlying map.
// Since this is a test we rather dubiously have a perfectly straight
// river through the middle & a perfectly straight coastline at the top.
// It's anticipated that in real usage this will be somewhat .. more complex.
type testOutline struct{}

func (t *testOutline) CanBuildOn(x, y int) bool {
	// coast line y <= 50
	if y <= 50 {
		return false
	}
	return !t.CanBridgeOver(x, y)
}

// CanBridgeOver
func (t *testOutline) CanBridgeOver(x, y int) bool {
	// ie. a river runs through the centre
	if x >= 495 && x <= 505 && y > 50 {
		return true
	}
	return false
}

// SuitableDock or "is this adjacent to a viable waterway"
func (t *testOutline) SuitableDock(x, y int) bool {
	if y == 50 { // dock is suitable on y = 50 (ie. coastline)
		return true
	}
	return false
}

func main() {
	fmt.Println("citygraph: building test map")

	bcfg := defaultConfig()
	cfg := &citygraph.CityConfig{
		Area:             image.Rect(0, 0, 1000, 1000), // area of entire city
		MainRoadWidth:    4,                            // width of main roads (between districts)
		MaxBridges:       -1,                           // any number of bridges for main roads
		MaxBridgeLength:  15,                           // keeping the max smallish prevents wacky diagonal bridges
		MinBridgeLength:  10,                           // ideally set to min river width
		MinDistrictSize:  150,                          // min "buildable" pixels in a district to aim for (approx)
		DesiredDistricts: 100,                          // how many districts we want to end up with
		MinDockSize:      10,                           // min "suitable dock" pixels to mark a district as "Docks"
		Fortifications: &citygraph.FortificationSettings{ // optional, configures city walls
			MaxBridgeWallLength:  0,                      // how far a wall can span over "bridgeable" pixels
			MinFortifiedSites:    6,                      // the number of districts we want within the city walls
			MaxCityGates:         2,                      // number of city gates
			MinDistBetweenTowers: 10,                     // (approx) pixels between most towers
			TowerArea:            image.Rect(0, 0, 5, 5), // area of a tower (as in, along walls)
			GatehouseArea:        image.Rect(0, 0, 8, 8), // area of a gatehouse (minus flanking towers)
			WallWidth:            5,                      // width of main city walls
			CurtainWallWidth:     4,                      // width of walls surrounding single districts
			WallBorderRoadWidth:  3,                      // width of road(s) that run along side walls/towers
		},
	}

	cg, err := citygraph.New(bcfg, cfg, &testOutline{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("==stats==\nDistrictsByType: %v\n\n", cg.Stats.DistrictsByType)
	total := map[int]int{}
	for _, d := range cg.Districts {
		for id, count := range d.Stats.BuildingsByID {
			n, _ := total[id]
			total[id] = n + count
		}

		fmt.Printf(
			"\t %s district (%d, %v):\n\t\tBuildings (sans central):%d Roads:%d\n\t\tWalls:%d Towers:%d Gates:%d\n\t\tBuildingsByID:%v\n\n",
			d.Type, d.ID, d.Site,
			len(d.Buildings),
			len(d.Roads),
			len(d.Walls), len(d.Towers), len(d.Gates),
			d.Stats.BuildingsByID,
		)
	}
	fmt.Printf("\n==total==\n\tBuildingsByID:%v\n", total)

	cm := cg.Map()

	err = cm.SaveAdv(fmt.Sprintf("citygraph.%d.png", cg.Seed), citygraph.DefaultScheme())
	if err != nil {
		panic(err)
	}

	err = cg.SaveJSON(fmt.Sprintf("citygraph.%d.json", cg.Seed))
	if err != nil {
		panic(err)
	}

	fmt.Printf("wrote citygraph.%d.json citygraph.%d.png\n", cg.Seed, cg.Seed)
}

// defaultConfig sets up some reasonable defaults for our test city.
// Nothing here is particularly special, they just seem sane to me :shrug:
func defaultConfig() *citygraph.BuilderConfig {
	c := &citygraph.BuilderConfig{
		Districts: map[citygraph.DistrictType]*citygraph.DistrictConfig{},
	}

	small := &citygraph.BuildingConfig{
		Area:        image.Rect(0, 0, 8, 8),
		Probability: 0.20,
		ID:          1, // nb. building IDs must be > 0
	}

	// whack in all district types
	// buildings here are just for demo purposes
	for _, d := range citygraph.AllDistrictTypes() {
		c.Districts[d] = &citygraph.DistrictConfig{
			// for each district type we start with some basic config
			RoadWidth:       2, // width of roads within district
			RoadDensity:     1.0,
			BuildingDensity: 1.0,
			Probability:     0.05, // probability this district type is chosen
			Buildings: []*citygraph.BuildingConfig{ // building footprint sizes
				small,
				&citygraph.BuildingConfig{
					Area:        image.Rect(0, 0, 9, 9),
					Probability: 0.10,
					ID:          2,
				},
				&citygraph.BuildingConfig{
					Area:        image.Rect(0, 0, 12, 12),
					Probability: 0.05,
					ID:          3,
				},
			},
		}
	}

	tiny := &citygraph.BuildingConfig{
		Area:        image.Rect(0, 0, 5, 5),
		Probability: 0.50,
		ID:          4,
	}

	huge := &citygraph.BuildingConfig{
		Area:        image.Rect(0, 0, 16, 16),
		Probability: 0.3,
		ID:          5,
	}

	giant := &citygraph.BuildingConfig{
		Area:        image.Rect(0, 0, 30, 30),
		Probability: 0.2,
		ID:          6,
	}

	// configure some defaults for specific district types

	// add larger / smaller / no buildings for various district types
	c.Districts[citygraph.ResidentialSlum].Buildings = append(c.Districts[citygraph.ResidentialSlum].Buildings, tiny)
	c.Districts[citygraph.Abandoned].Buildings = append(c.Districts[citygraph.Abandoned].Buildings, tiny)
	c.Districts[citygraph.ResidentialMiddle].Buildings = append(c.Districts[citygraph.ResidentialMiddle].Buildings, tiny)
	c.Districts[citygraph.Docks].Buildings = append(c.Districts[citygraph.Docks].Buildings, tiny)
	c.Districts[citygraph.Graveyard].Buildings = []*citygraph.BuildingConfig{tiny}
	c.Districts[citygraph.Empty].Buildings = nil
	c.Districts[citygraph.Park].Buildings = nil
	c.Districts[citygraph.Square].Buildings = nil
	c.Districts[citygraph.Market].Buildings = nil
	c.Districts[citygraph.Fields].Buildings = []*citygraph.BuildingConfig{giant, huge}
	c.Districts[citygraph.ResidentialUpper].Buildings = append(c.Districts[citygraph.ResidentialUpper].Buildings, giant, huge)
	c.Districts[citygraph.Civic].Buildings = append(c.Districts[citygraph.Civic].Buildings, giant, huge)
	c.Districts[citygraph.Fortress].Buildings = append(c.Districts[citygraph.Fortress].Buildings, giant, huge)
	c.Districts[citygraph.Temple].Buildings = append(c.Districts[citygraph.Temple].Buildings, giant, huge)

	// specify that fortress districts have their own walls / towers / gate
	c.Districts[citygraph.Fortress].HasCurtainFortifications = true

	// set "central" buildings - these are placed near the district centres
	c.Districts[citygraph.Fortress].Central = huge
	c.Districts[citygraph.Civic].Central = huge
	c.Districts[citygraph.Temple].Central = huge
	c.Districts[citygraph.Square].Central = small

	// configure min / max counts of various types of districts
	c.Districts[citygraph.Civic].MinInCity = 1
	c.Districts[citygraph.Civic].MaxInCity = 1
	c.Districts[citygraph.Temple].MinInCity = 1
	c.Districts[citygraph.Temple].MaxInCity = 1
	c.Districts[citygraph.Fortress].MaxInCity = 1
	c.Districts[citygraph.Graveyard].MinInCity = 1
	c.Districts[citygraph.Graveyard].MaxInCity = 3
	c.Districts[citygraph.Industrial].MaxInCity = 5
	c.Districts[citygraph.Research].MaxInCity = 1
	c.Districts[citygraph.Prison].MaxInCity = 1
	c.Districts[citygraph.Barracks].MaxInCity = 4
	c.Districts[citygraph.Park].MinInCity = 1
	c.Districts[citygraph.Park].MaxInCity = 1
	c.Districts[citygraph.Abandoned].MaxInCity = 1
	c.Districts[citygraph.Docks].MaxInCity = 2
	c.Districts[citygraph.Market].MinInCity = 1
	c.Districts[citygraph.Market].MaxInCity = 1
	c.Districts[citygraph.Square].MinInCity = 1
	c.Districts[citygraph.Square].MaxInCity = 1

	// fiddle with road / building density / road width for various types
	c.Districts[citygraph.Fortress].RoadDensity = 0.2
	c.Districts[citygraph.Market].RoadDensity = 0.4
	c.Districts[citygraph.Square].RoadDensity = 0.2
	c.Districts[citygraph.Park].RoadDensity = 0.0 // no roads
	c.Districts[citygraph.Fields].RoadDensity = 0.1
	c.Districts[citygraph.Fields].RoadWidth = 2
	c.Districts[citygraph.ResidentialSlum].RoadWidth = 1
	c.Districts[citygraph.ResidentialSlum].RoadDensity = 1
	c.Districts[citygraph.ResidentialMiddle].RoadDensity = 0.7
	c.Districts[citygraph.ResidentialUpper].RoadWidth = 3
	c.Districts[citygraph.ResidentialUpper].RoadDensity = 0.3
	c.Districts[citygraph.Civic].RoadWidth = 3
	c.Districts[citygraph.Civic].RoadDensity = 0.7
	c.Districts[citygraph.Temple].RoadDensity = 0.7
	c.Districts[citygraph.Abandoned].RoadWidth = 1
	c.Districts[citygraph.Abandoned].RoadDensity = 1.2
	c.Districts[citygraph.Warehouse].RoadDensity = 0.5
	c.Districts[citygraph.Empty].RoadDensity = 0.0
	c.Districts[citygraph.ResidentialUpper].BuildingDensity = 0.7
	c.Districts[citygraph.ResidentialMiddle].BuildingDensity = 0.9
	c.Districts[citygraph.Civic].BuildingDensity = 0.8
	c.Districts[citygraph.Temple].BuildingDensity = 0.8
	c.Districts[citygraph.Research].BuildingDensity = 0.7

	// annnd finally adjust probabilities of us randomly picking various types
	c.Districts[citygraph.Research].Probability = 0.01
	c.Districts[citygraph.Barracks].Probability = 0.03
	c.Districts[citygraph.Prison].Probability = 0.01
	c.Districts[citygraph.Fields].Probability = 0.3
	c.Districts[citygraph.Empty].Probability = 0.1
	c.Districts[citygraph.ResidentialSlum].Probability = 0.1
	c.Districts[citygraph.ResidentialLower].Probability = 0.4
	c.Districts[citygraph.ResidentialMiddle].Probability = 0.2
	c.Districts[citygraph.ResidentialUpper].Probability = 0.01

	return c
}
