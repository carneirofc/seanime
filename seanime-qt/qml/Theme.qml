pragma Singleton
import QtQuick

// Central design tokens for the whole app. One source of truth for colors,
// radii, spacing, type scale, and motion so every component stays consistent
// and a restyle is a one-file change.
//
// Palette is matched to the Seanime web frontend's default theme
// (background #070707, indigo brand accent #6152df). Ports to C++ Qt verbatim
// as a QML singleton (registered the same way via a qmldir).
QtObject {
    id: theme

    // ---- surfaces (dark, stepped from the window background upward) ----
    readonly property color bg:            "#070707"   // window background
    readonly property color surface:       "#121218"   // default panels / cards
    readonly property color surfaceAlt:    "#17171f"   // toolbar / sidebar chrome
    readonly property color surfaceHover:  "#20202b"   // hovered card / row
    readonly property color elevated:      "#2a2a38"   // badges, nav-active, chip fill
    readonly property color inset:         "#0d0d12"   // image wells behind posters

    // ---- borders ----
    readonly property color border:        "#26262f"
    readonly property color borderStrong:  "#3a3a48"

    // ---- text ----
    readonly property color textStrong:    "#ffffff"
    readonly property color text:          "#e6e6ee"
    readonly property color textDim:       "#c0c0cc"
    readonly property color textMuted:     "#8a8a96"

    // ---- brand accent ----
    readonly property color accent:        "#6152df"
    readonly property color accentHover:   "#7264e6"
    readonly property color accentText:    "#ffffff"
    readonly property color accentSoft:    "#a99fff"   // link-like text on dark
    readonly property color accentFill:    "#241f45"   // low-emphasis accent surface

    // ---- status ----
    readonly property color success:       "#3ecf5b"
    readonly property color successFill:    "#173a22"
    readonly property color successText:    "#8fe6a3"
    readonly property color warning:        "#e0b341"
    readonly property color warnFill:       "#3a3320"
    readonly property color warnText:       "#ffd98f"
    readonly property color danger:         "#e05a5a"
    readonly property color dangerFill:     "#5a1f1f"
    readonly property color dangerText:     "#ffd9d9"

    // ---- misc ----
    readonly property color overlay:        "#cc000000"  // poster badge scrim
    readonly property color shadow:         "#000000"

    // ---- radii ----
    readonly property int radiusSm:  4
    readonly property int radius:    8
    readonly property int radiusLg:  12
    readonly property int radiusPill: 999

    // ---- spacing ----
    readonly property int spacingXs: 4
    readonly property int spacingSm: 8
    readonly property int spacing:   12
    readonly property int spacingLg: 16
    readonly property int spacingXl: 24

    // ---- type scale (pixel sizes) ----
    readonly property int fontXs:   11
    readonly property int fontSm:   12
    readonly property int fontMd:   13
    readonly property int fontBase: 14
    readonly property int fontLg:   16
    readonly property int fontXl:   18
    readonly property int fontXxl:  20
    readonly property int fontHero: 24

    // ---- motion ----
    readonly property int durFast: 120   // hover / color state changes
    readonly property int durBase: 200   // most transitions
    readonly property int durSlow: 320   // page changes, entrances

    // Easing curves (Easing enum values; used as `easing.type: Theme.easeStandard`).
    readonly property int easeStandard: Easing.OutCubic   // general purpose
    readonly property int easeEmphasis: Easing.OutBack    // playful lift/scale
    readonly property int easeInOut:    Easing.InOutQuad  // symmetric

    // ---- effect tuning ----
    readonly property real cardLift:     1.03   // hover scale for poster cards
    readonly property real shadowBlur:   0.55   // MultiEffect blur (0..1)
    readonly property real shadowBlurHi: 0.85   // hovered elevation
}
