package citygraph

// Outline tells citygrapher roughly what is at a given location.
// We only have three questions;
// - can I build on it? (place roads, buildings, towers, gatehouses etc)
// - can I bridge over it? (place bridges, walls - assuming configured min/max lengths)
// - is the square suitable for a "docks" district? (generally: adjacent to the sea, large river)
type Outline interface {
	// true if we can place buildings on this space
	CanBuildOn(x, y int) bool

	// true if we can build bridges over this space (implies CanBuildOn false
	// otherwise we don't need to bridge ..)
	CanBridgeOver(x, y int) bool

	// true if this could be used for a dock district typically implies it sits
	// alongside a river / sea but .. who knows.
	SuitableDock(x, y int) bool
}
