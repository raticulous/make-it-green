# Make It Green icon concepts

- A. Sprout check: a thick green checkmark whose long arm ends in a small leaf ("make it green" literally). Charming at 64 px, but the leaf turns to mush at 16 px and there's no solid shape for the off state to hollow out.
- B. Pulse squircle: a green rounded square holding a heartbeat line ("awake"). Reads well, but it looks like a health/monitoring app and drops the checkmark.
- C. Leaf badge (chosen): the status light is a diagonal leaf instead of a circle, with a bold white check in it.

Chosen: C. It keeps the "available, all good" check but has its own silhouette (a pointed leaf, not the round Teams badge), and the leaf carries the "green" name. It is one solid shape plus one stroke, so it stays crisp at 16 px. The off state is the same leaf as a mid-grey (#8c8c8c) outline with a grey check, visible on both light and dark taskbars and clearly "inactive" next to the filled on state.
Tray: on is the filled green leaf with a white check, paused is a plain yellow leaf, off is a faded grey outline. All three sit at 80% of the canvas with thin strokes, so they match the size of other tray icons.
The app icon is the on-state leaf with a gradient and a small stem, on a dark rounded-square plate.
macOS tray: monochrome template leaves (`tray-*-template.svg`), full size, so the menu bar tints them like its other icons. On cuts the check out of the solid leaf, paused is the plain solid leaf, off is the outline. Render to 32 px PNGs in `cmd/make-it-green-gui/icons/`.
