package voronoi

// CandidateFilter accepts or rejects a candidate point (x, y) based
// purely on the given (x, y).
// These filters are run before SiteFilter(s) which naturally require
// us to iterate each site.
type CandidateFilter func(x, y int) bool

// SiteFilter is a filter for a candidate (x, y) point that is run
// against every current Site in the builder.
// Ie. we must 'accept' the candidate point (x, y) when compared
// with every existing Site that we've previously accepted.
type SiteFilter func(ax, ay, sx, sy int) bool

// MinDistance ensures that a candidate (x, y) point is at least `dist`
// distance away from every other site.
func (b *Builder) MinDistance(dist float64) SiteFilter {
	return func(ax, ay, sx, sy int) bool {
		return calculateDist(sx, sy, ax, ay) >= dist
	}
}
