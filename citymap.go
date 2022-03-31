package citygraph

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/voidshard/citygraph/internal/encoding"

	"github.com/boljen/go-bitmap"
	"github.com/fogleman/gg"
	"golang.org/x/image/colornames"
)

const (
	// bit numbers for our bitmap
	bitRoad   = 0
	bitBridge = 1
	bitWall   = 2
	bitTower  = 3
	bitGate   = 4
)

// CityMap is a graphical representation of a CityGraph
type CityMap interface {
	// Save as custom file in a format defined by the library
	Save(fpath string) error

	// SaveAdv saves as an image with the given color scheme
	SaveAdv(fpath string, scheme *ColourScheme) error

	// CustomImage returns an image with the given color scheme
	CustomImage(scheme *ColourScheme) (image.Image, error)

	// only one of these things can be at a given (x,y) co-ord
	IsRoad(x, y int) bool
	IsBridge(x, y int) bool
	IsWall(x, y int) bool
	IsTower(x, y int) bool
	IsGatehouse(x, y int) bool
	BuildingID(x, y int) (int, error)

	// internal helper for IsWall || IsTower || IsGatehouse
	isFortification(x, y int) bool

	// District returns the district type & id at the given x,y
	District(x, y int) (DistrictType, int, error)
}

// imageMap is a particular implementation of CityMap using a RGBA64
type imageMap struct {
	// Map is an RGBA64 image where each pixel of 64 bits is split via
	//
	// nb. helpful
	//  8  bits 0-255
	//  16 bits 0-65,535
	//  32 bits 0-4,294,967,295
	//
	// R [16 bits]
	//   16-1: [16 bits] -> district id
	// G [16 bits]
	// B [16 bits]
	//   32-1 [32 bits] -> building id, G holds the significant bits
	// A [16 bits]
	//   16-9 [8 bits] -> district type
	//    8-1 [8 bits] -> bitmap (true if set, false if not)
	//       bit 0 -> isRoad
	//       bit 1 -> isBridge
	//       bit 2 -> isWall
	//       bit 3 -> isTower
	// 	 bit 4 -> isGatehouse
	//       bit 5-7 -> unused
	//
	im *image.RGBA64

	// temporary map for road / wall / tower / gatehouse network
	// We draw the shapes with a drawing lib because it's 100x easier
	// than figuring out all the geometry ourselves .. we then transfer
	// this information into our main working image in endDraw()
	ctx *gg.Context

	// mask for our drawing context. We mask out some areas as
	// required so we cannot paint over them in later stages
	mask *image.Alpha
}

// ColourScheme defines how various features in a city should be coloured.
type ColourScheme struct {
	Roads     color.Color
	Walls     color.Color
	Towers    color.Color
	Gates     color.Color
	Bridges   color.Color
	Buildings color.Color
	Districts map[DistrictType]color.Color
}

// DefaultScheme returns a reasonable default ColourScheme.
func DefaultScheme() *ColourScheme {
	return &ColourScheme{
		Roads:     colornames.Dimgray,
		Bridges:   colornames.Darkgray,
		Walls:     colornames.Black,
		Towers:    colornames.Crimson,
		Gates:     colornames.Black,
		Buildings: colornames.Black,
		Districts: map[DistrictType]color.Color{
			Park:              colornames.Lightgreen,
			Temple:            colornames.Gold,
			Civic:             colornames.Indigo,
			Graveyard:         colornames.Lightgray,
			ResidentialUpper:  colornames.Royalblue,
			ResidentialMiddle: colornames.Steelblue,
			ResidentialLower:  colornames.Slateblue,
			ResidentialSlum:   colornames.Darkslateblue,
			Abandoned:         colornames.Whitesmoke,
			Fortress:          colornames.Crimson,
			Market:            colornames.Fuchsia,
			Commercial:        colornames.Hotpink,
			Square:            colornames.Yellow,
			Industrial:        colornames.Firebrick,
			Warehouse:         colornames.Brown,
			Barracks:          colornames.Maroon,
			Prison:            colornames.Grey,
			Docks:             colornames.Lightblue,
			Research:          colornames.Mediumturquoise,
			Fields:            colornames.Wheat,
			Empty:             colornames.White,
		},
	}
}

func (c *imageMap) setBuilding(x, y int, b *BuildingConfig) {
	if b == nil {
		return
	}
	for dx := b.Area.Min.X; dx < b.Area.Max.X; dx++ {
		for dy := b.Area.Min.Y; dy < b.Area.Max.Y; dy++ {
			c.setBuildingID(x+dx, y+dy, b.ID)
		}
	}
}

// Save the CityMap as is to disk
func (c *imageMap) Save(fpath string) error {
	return savePNG(fpath, c.im)
}

// CustomImage returns the CityMap coloured with the given Scheme
func (c *imageMap) CustomImage(scheme *ColourScheme) (image.Image, error) {
	bnds := c.im.Bounds()
	im := image.NewRGBA(bnds)

	for dy := bnds.Min.Y; dy < bnds.Max.Y; dy++ {
		for dx := bnds.Min.X; dx < bnds.Max.X; dx++ {
			bm := c.getBM(dx, dy)

			if bm.Get(bitGate) {
				im.Set(dx, dy, scheme.Gates)
				continue
			} else if bm.Get(bitTower) {
				im.Set(dx, dy, scheme.Towers)
				continue
			} else if bm.Get(bitWall) {
				im.Set(dx, dy, scheme.Walls)
				continue
			} else if bm.Get(bitBridge) {
				im.Set(dx, dy, scheme.Bridges)
				continue
			} else if bm.Get(bitRoad) {
				im.Set(dx, dy, scheme.Roads)
				continue
			}

			buildingID, err := c.BuildingID(dx, dy)
			if err != nil {
				return nil, err
			}
			if buildingID != 0 {
				im.Set(dx, dy, scheme.Buildings)
				continue
			}

			dtype, _, err := c.District(dx, dy)
			if err != nil {
				return nil, err
			}
			col, ok := scheme.Districts[dtype]
			if ok {
				im.Set(dx, dy, col)
			}
		}
	}

	return im, nil
}

// SaveAdv essentially saves the CityMap using the given scheme to disk.
// Essentially sugar around "CustomImage()" followed by writing out a PNG.
func (c *imageMap) SaveAdv(fpath string, scheme *ColourScheme) error {
	im, err := c.CustomImage(scheme)
	if err != nil {
		return err
	}
	ctx := gg.NewContextForRGBA(im.(*image.RGBA))
	return ctx.SavePNG(fpath)
}

// District returns the district type & ID at x,y
func (c *imageMap) District(x, y int) (DistrictType, int, error) {
	if c.isOutOfBounds(x, y) {
		return Empty, -1, fmt.Errorf("(%d,%d) is out of bounds", x, y)
	}

	v := c.im.RGBA64At(x, y)
	typeid, _ := encoding.Split16(v.A)

	return districtForID(int(typeid)), int(v.R), nil
}

// BuildingID returns the buildingID (see BuildingConfig.ID) at x,y
// A value of 0 indicates that there is no building.
func (c *imageMap) BuildingID(x, y int) (int, error) {
	if c.isOutOfBounds(x, y) {
		return -1, fmt.Errorf("(%d,%d) is out of bounds", x, y)
	}

	v := c.im.RGBA64At(x, y)
	return int(encoding.Merge16(v.G, v.B)), nil
}

// setBuildingID sets the given ID at x,y
func (c *imageMap) setBuildingID(x, y, id int) {
	v := c.im.RGBA64At(x, y)
	g, b := encoding.Split32(uint32(id))
	v.G = g
	v.B = b
	c.im.SetRGBA64(x, y, v)
}

// setDistrict sets the given type & id at x,y
func (c *imageMap) setDistrict(x, y int, typ DistrictType, id int) error {
	denum := typ.ID()

	v := c.im.RGBA64At(x, y)
	_, bmbits := encoding.Split16(v.A)

	v.R = uint16(id)
	v.A = encoding.Merge8(uint8(denum), bmbits)

	c.im.SetRGBA64(x, y, v)
	return nil
}

// setRoad sets x,y as Road
func (c *imageMap) setRoad(x, y int) {
	bm := c.getBM(x, y)
	bm.Set(bitRoad, true)
	c.setBM(x, y, bm)
}

// setBridge sets x,y as bridge
func (c *imageMap) setBridge(x, y int) {
	bm := c.getBM(x, y)
	bm.Set(bitBridge, true)
	c.setBM(x, y, bm)
}

// setBM sets the 8 bit bitmap at x,y
func (c *imageMap) setBM(x, y int, bm bitmap.Bitmap) {
	num := encoding.FromBytes8(bm.Data(true))

	current := c.im.RGBA64At(x, y)
	dtype, _ := encoding.Split16(current.A)
	current.A = encoding.Merge8(dtype, num)

	c.im.SetRGBA64(x, y, current)
}

// getBM gets the 8 bit bitmap at x,y
func (c *imageMap) getBM(x, y int) bitmap.Bitmap {
	current := c.im.RGBA64At(x, y)

	_, bmdata := encoding.Split16(current.A)
	data := encoding.ToBytes8(bmdata)
	return bitmap.Bitmap(data)
}

// isFortification returns if x,y is any of Tower, Gate, Wall
func (c *imageMap) isFortification(x, y int) bool {
	_, _, b, _ := c.ctx.Image().At(x, y).RGBA()
	return b > 0
}

// IsWall returns if there is a wall at x,y
func (c *imageMap) IsWall(x, y int) bool {
	if c.isOutOfBounds(x, y) {
		return false
	}
	return c.getBM(x, y).Get(bitWall)
}

// IsTower returns if there is a tower at x,y
func (c *imageMap) IsTower(x, y int) bool {
	if c.isOutOfBounds(x, y) {
		return false
	}
	return c.getBM(x, y).Get(bitTower)
}

// IsGatehouse returns if there is a gatehouse at x,y
func (c *imageMap) IsGatehouse(x, y int) bool {
	if c.isOutOfBounds(x, y) {
		return false
	}
	return c.getBM(x, y).Get(bitGate)
}

// IsRoad returns if there is a road at x,y
func (c *imageMap) IsRoad(x, y int) bool {
	if c.isOutOfBounds(x, y) {
		return false
	}
	return c.getBM(x, y).Get(bitRoad)
}

// IsBridge returns if there is a bridge at x,y
func (c *imageMap) IsBridge(x, y int) bool {
	if c.isOutOfBounds(x, y) {
		return false
	}
	return c.getBM(x, y).Get(bitBridge)
}

// isOutOfBounds determines if x,y is outside of the image area
func (c *imageMap) isOutOfBounds(x, y int) bool {
	bnds := c.im.Bounds()
	return x < bnds.Min.X || x > bnds.Max.X || y < bnds.Min.Y || y > bnds.Max.Y
}

// nearbyFortifications returns of any pixels within radius of x,y in `im`
// are wall, gatehouse or tower
func nearbyFortifications(x, y, radius int, im image.Image) bool {
	for dy := y - radius; dy < y+radius; dy++ {
		for dx := x - radius; dx < x+radius; dx++ {
			_, _, b, _ := im.At(dx, dy).RGBA()
			if b >= 50 {
				return true
			}
		}
	}
	return false
}

// endDraw copies our rough road / wall outline sketch to our proper map.
// Basically we paint roads / walls / towers / gatehouses because it's
// much easier to lean on a graphics library & then copy the results
// over to our main image later.
func (c *imageMap) endDraw(wallBorderRoadWidth int, outline Outline) {
	temp := c.ctx.Image()
	bnds := temp.Bounds()

	for dy := bnds.Min.Y; dy < bnds.Max.Y; dy++ {
		for dx := bnds.Min.X; dx < bnds.Max.X; dx++ {
			r, g, b, _ := temp.At(dx, dy).RGBA()
			r = r >> 8
			g = g >> 8
			b = b >> 8

			bm := bitmap.New(8)

			if r == 0 && g == 0 && b == 0 {
				road := outline.CanBuildOn(dx, dy)
				water := outline.CanBridgeOver(dx, dy)
				if (road || water) && nearbyFortifications(dx, dy, wallBorderRoadWidth, temp) {
					bit := bitRoad
					if water {
						bit = bitBridge
					}
					bm.Set(bit, true)
				} else {
					continue
				}
			} else {
				if g > 0 {
					bm.Set(bitBridge, true) // bridge
				}

				if b >= 150 {
					bm.Set(bitGate, true)
				} else if b >= 100 {
					bm.Set(bitTower, true)
				} else if b >= 50 {
					bm.Set(bitWall, true)
				}

				if r > 0 {
					bm.Set(bitRoad, true) // road
				}
			}

			c.setBM(dx, dy, bm)
		}
	}
}

// drawGatehouse (rect) on to our scratch image
func (c *imageMap) drawGatehouse(a image.Point, width, height int) {
	c.ctx.SetColor(color.RGBA{0, 0, 150, 255})
	c.ctx.DrawRectangle(float64(a.X), float64(a.Y), float64(width), float64(height))
	c.ctx.Fill()
	// mark area as non-drawable
	draw.Draw(c.mask, image.Rect(a.X, a.Y, a.X+width, a.Y+height), image.Transparent, image.ZP, draw.Src)
}

// drawTower (rect) on to our scratch image
func (c *imageMap) drawTower(a image.Point, width, height int) {
	c.ctx.SetColor(color.RGBA{0, 0, 100, 255})
	c.ctx.DrawRectangle(float64(a.X), float64(a.Y), float64(width), float64(height))
	c.ctx.Fill()
	// mark area as non-drawable
	draw.Draw(c.mask, image.Rect(a.X, a.Y, a.X+width, a.Y+height), image.Transparent, image.ZP, draw.Src)
}

// drawWall (line) on to our scratch image
func (c *imageMap) drawWall(a, b image.Point, width int) {
	c.ctx.SetColor(color.RGBA{0, 0, 50, 255})
	c.ctx.SetLineWidth(float64(width))
	c.ctx.SetLineCapSquare()
	c.ctx.DrawLine(float64(a.X), float64(a.Y), float64(b.X), float64(b.Y))
	c.ctx.Stroke()
}

// drawBridge (line) on to our scratch image
func (c *imageMap) drawBridge(a, b image.Point, width int) {
	c.ctx.SetColor(color.RGBA{0, 255, 0, 255})
	c.ctx.SetLineCapSquare()
	c.ctx.SetLineWidth(float64(width))
	c.ctx.DrawLine(float64(a.X), float64(a.Y), float64(b.X), float64(b.Y))
	c.ctx.Stroke()
}

// drawRoad (line) on to our scratch image
func (c *imageMap) drawRoad(a, b image.Point, width int) {
	c.ctx.SetColor(color.RGBA{255, 0, 0, 255})
	c.ctx.SetLineCapSquare()
	c.ctx.SetLineWidth(float64(width))
	c.ctx.DrawLine(float64(a.X), float64(a.Y), float64(b.X), float64(b.Y))
	c.ctx.Stroke()
}

// newMap returns a new map with the given bounds
func newMap(bounds image.Rectangle) *imageMap {
	ctx := gg.NewContextForRGBA(image.NewRGBA(bounds))
	ctx.SetRGBA(0, 0, 0, 0)
	ctx.Clear()

	// we use a mask to forbid drawing over our towers / gatehouses with
	// walls / roads. Since otherwise we can't ensure that thick lines
	// ending at the tower/gatehouse edge wont eat into our shapes
	// and it's waaaay more annoying drawing walls / roads first.
	mask := image.NewAlpha(bounds)
	draw.Draw(mask, mask.Bounds(), image.Opaque, image.ZP, draw.Src)
	ctx.SetMask(mask)

	return &imageMap{
		ctx:  ctx,
		im:   image.NewRGBA64(bounds),
		mask: mask,
	}
}
