package voronoi

import (
	"image"
	"math"
	"math/rand"
	"time"
)

// Builder struct makes managing the setup of a voronoi diagram easier.
// We're interested here in building a voronoi diagram with some structure
// to how 'sites' (centres of voronoi cells) are laid out.
type Builder struct {
	bounds image.Rectangle
	sites  []image.Point
	rng    *rand.Rand
	sfilt  []SiteFilter
	cfilt  []CandidateFilter
}

// NewBuilder returns a new Voronoi diagram builder
func NewBuilder(bounds image.Rectangle) *Builder {
	return &Builder{
		bounds: bounds,
		sites:  []image.Point{},
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SiteCount returns how many sites we've currently got configured
func (b *Builder) SiteCount() int {
	return len(b.sites)
}

// Voronoi returns the Voronoi diagram given our current sites.
// Nb. there must be at least one site set or this will panic.
func (b *Builder) Voronoi() *Voronoi {
	if len(b.sites) == 0 {
		panic("voronoi diagram requires at least one site")
	}
	return newVoronoi(b)
}

// SetSeed sets our internal RNG seed
func (b *Builder) SetSeed(seed int64) {
	b.rng = rand.New(rand.NewSource(seed))
}

// SetCandidateFilters sets filters that accept / reject a proposed site without
// reference to other currently set site(s).
func (b *Builder) SetCandidateFilters(f ...CandidateFilter) {
	b.cfilt = f
}

// SetSiteFilters sets filters that compare proposed sites to all current sites.
func (b *Builder) SetSiteFilters(f ...SiteFilter) {
	b.sfilt = f
}

// AddRandomSite places a site at random, assuming it obeys all currently set filters.
func (b *Builder) AddRandomSite() (int, int, int, bool) {
	// make a random point within bounds
	candidateX := b.rng.Intn(b.bounds.Max.X-b.bounds.Min.X) + b.bounds.Min.X
	candidateY := b.rng.Intn(b.bounds.Max.Y-b.bounds.Min.Y) + b.bounds.Min.Y

	if !b.accepted(candidateX, candidateY) {
		return 0, 0, 0, false
	}

	return candidateX, candidateY, b.addSite(candidateX, candidateY), true
}

// AddSite places a site at the given location, assuming it obeys currently set filters.
func (b *Builder) AddSite(x, y int) (int, bool) {
	if !b.accepted(x, y) {
		return 0, false
	}
	return b.addSite(x, y), true
}

// accepted returns if the proposed site location (x, y) is acceptable to our filters.
// We run CandidateFilter(s) first so we can hopefully reject candidates early.
func (b *Builder) accepted(candidateX, candidateY int) bool {
	// first check if we can reject early with a CandidateFilter
	if b.cfilt != nil {
		for _, fn := range b.cfilt {
			if !fn(candidateX, candidateY) {
				return false
			}
		}
	}

	// check if we can reject with any SiteFilter, for every site
	if b.sfilt != nil {
		for _, s := range b.sites {
			for _, fn := range b.sfilt {
				if !fn(candidateX, candidateY, s.X, s.Y) {
					return false
				}
			}
		}
	}

	return true
}

// calculateDist standard pythag.
func calculateDist(ax, ay, bx, by int) float64 {
	return math.Sqrt(math.Pow(float64(ax-bx), 2) + math.Pow(float64(ay-by), 2))
}

// addSite adds a site, no filters are run.
func (b *Builder) addSite(x, y int) int {
	id := len(b.sites)
	b.sites = append(b.sites, image.Pt(x, y))
	return id
}
