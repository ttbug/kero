# Icons

- `f7-*.svg`: [Framework7 Icons](https://framework7.io/icons/) (MIT, © Vladimir Kharlampidi), drawn after Apple's SF Symbols.
  `f7-doc-badge-plus.svg` is two of them, as Framework7 has no `doc.badge.plus`: `doc`, masked around the badge of `folder-badge-plus`, moved to its lower corner, as SF Symbols draw it.
- `meteor-*.svg`: [Meteor Icons](https://github.com/zkreations/icons) (MIT, © zkreations).
- `si-*.svg`: [Simple Icons](https://simpleicons.org) (CC0), programs' logos; the brands are their owners' trademarks.
- `pi.svg`: [Pi](https://pi.dev) (MIT, © Earendil), scaled to `24×24` from its mark on `pi.dev`.
- `octicon-*.svg`: [Octicons](https://primer.style/octicons) (MIT, © GitHub), for what Framework7 has no icon of, as a file's diff and wrapping its lines.
- `lucide-*.svg`: [Lucide](https://lucide.dev) (ISC, © Lucide Contributors), with a heavier stroke than Iconify serves (`stroke-width`), as SF Symbols' semibold: 2.5 for `chevrons-up-down`, 3 for `check`.
- `antigravity.png`: official color emblem for Google Antigravity (`agy`).
- `antigravity.svg`: monochrome silhouette derived from the official Google Antigravity emblem.

- `files/`: [Pierre Icons](https://diffs.com/icons) (MIT, © The Pierre Computer Company, `files/LICENSE.md`), the file and folder icons of the Files and Changes trees: the complete theme, in its colors for the light appearance (`*_light.svg`) and the dark one. `go run ./scripts/fileicons <checkout>` brings them from a checkout of [pierrecomputer/vscode-icons](https://github.com/pierrecomputer/vscode-icons) built with `node scripts/build-icon-theme.mjs`, with the names and extensions they are for (`fileicons_gen.go`).

All but `files/` as [Iconify](https://iconify.design) serves them: `https://api.iconify.design/f7/<name>.svg`.

`tray-template.png` is the simplified three-piece X from the user-provided Xview artwork, black on transparent for macOS to tint in the menu bar, centered in 18×18 points, at 2×. Its full-size source is `resources/xview-menubar.png`. The built-in imagegen edit prompt preserves the left chevron and two detached right arms while removing the tile, windows, internal details, color and glow. `tray.png` is the app's icon at 64×64, for the notification area of Windows and Linux.

`resources/icon.png` and `resources/icon-dev.png` use the user-provided second Xview artwork, exported at 1024×1024. Both builds currently share the same artwork. The built-in imagegen correction prompt preserves the cyan/orange/purple X and terminal artwork, removes the pre-rounded tile boundary and transparent padding, and extends the dark background to every edge of a fully opaque square. Every pixel must have alpha 255; macOS supplies the final corner mask.

On macOS 27.0.1, the previous padded, partly transparent bitmap was rendered by Icon Services as a small icon inside a broad white plate, even though the PNG and bundled ICNS had no white border. Apple describes automatic bitmap segmentation and recommends full-bleed opaque backgrounds and unmasked square layers in its [app icon guidelines](https://developer.apple.com/design/human-interface-guidelines/app-icons.md). Verify the built `.app` using `NSWorkspace.shared.icon(forFile:)` and render that image to a PNG; checking the source PNG or extracted ICNS alone does not cover the system-added plate. The corrected build was verified this way, with the dark tile filling the system mask and no white plate.
