# Citygraph

Create highly configurable orthogonal city layouts, semi or fully randomly, to export as JSON or render as images.

![example](https://raw.githubusercontent.com/voidshard/citygraph/main/assets/citygraph.1648738746739963194.png)


### Why

In particular citygraph is intended to make layouts for 2-d orthogonal cities - that is, to be rendered using square tiles (in future this may be extended). Secondly the city planner is highly configurable, you can specify what pixels are available for what, configure each district & building type, set down where you want particular districts to be ("a dock here please, with a fortress next to it") and many other things. Thirdly the lib can export everything you ever wanted to know - the location of every district, building, bridge, road, gate & tower. Finally citygraph is intended to dovetail nicely with larger procedural worldgen works.


### How

It's fairly straight forward, you provide something to fit the Outline interface
```golang
CanBuildOn(x, y int) bool
CanBridgeOver(x, y int) bool
SuitableDock(x, y int) bool
```
that simply tells the city planner 
- if a pixel is suitable for a building/road (ie. land)
- if a pixel can be bridged over (ie. generally a river)
- if a pixel is suitable for a Dock (ie. generally a sheltered harbour)
Otherwise a pixel might be none of these (it could be out of bounds, a lake of fire, a cliff face or ..whatever).

Then we provide two configs & our outline to the New function (see [config.go](https://github.com/voidshard/citygraph/blob/main/config.go) and the [example](https://github.com/voidshard/citygraph/blob/main/examples/testmap/main.go))
```golang
citygraph.New(&citygraph.BuilderConfig{}, &citygraph.CityConfig{}, myOutline)
```


### Notes

All buildings to citygraph are rectangles -- we don't care if it represents a full building, a building surrounded by a fence, a fountain, garden, statue or whatever else -- citygraph cares about how much space it takes up, where & how frequently it occurs.

Given the same configuration(s) and seed the output map is *nearly* the same. There is variation due to (I believe) rounding in libs we lean on (particularly around voronoi diagram edges / verticies)

Sometimes due to the above issue our edges don't align perfectly - interesting because often re-rendering fixes the issue. It's mostly noticable when our walls end up with a gap :awkward: (#TODO)

Currently the lib adds roads / bridges alongside walls to ensure all areas are reachable. This means that these bridges evade some of our usual checks with respect to max bridge lengths / counts (#TODO)

