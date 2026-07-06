pragma Singleton
import QtQuick

// Central design tokens for the whole app. One source of truth for colors,
// radii, spacing, type scale, and motion so every component stays consistent
// and a restyle is a one-file change.
//
// The four inputs at the top (uiScale, density, mode, accentBase) are the
// user-controllable knobs. They are plain (writable) properties that the QML
// root (Main.qml) pushes in from the persisted client prefs — a QML singleton
// can't read the `app` context property itself, so Main.qml bridges it. Every
// token below derives from these inputs, so changing one reflows the whole UI
// live. Ports to C++ Qt verbatim as a QML singleton (registered via qmldir).
QtObject {
    id: theme

    // ---- user-controlled appearance inputs (set by Main.qml from app prefs) ----
    property real uiScale: 1.0            // font/control scale (0.8–1.5)
    property real density: 1.0            // spacing scale (0.85 compact – 1.2 spacious)
    property string mode: "dark"          // "dark" | "light"
    property color accentBase: "#6152df"  // brand accent; the accent family derives from it
    property real posterScale: 1.0        // poster-grid card size (0.7–1.4)

    // ---- surfaces / borders / text (one inline ternary on `mode` per token).
    // Each token depends directly on the `mode` property, so a mode switch
    // re-evaluates every one. (An indirection through a `var` palette map is not
    // reliably reactive — QML doesn't track member reads on a JS object — so the
    // explicit per-token form is used instead.) Comments mark the dark value's
    // original role. ----
    //                                            light        dark
    readonly property color bg:            mode === "light" ? "#f5f5f8" : "#070707"  // window background
    readonly property color surface:       mode === "light" ? "#ffffff" : "#121218"  // default panels / cards
    readonly property color surfaceAlt:    mode === "light" ? "#ececf1" : "#17171f"  // toolbar / sidebar chrome
    readonly property color surfaceHover:  mode === "light" ? "#e3e3ea" : "#20202b"  // hovered card / row
    readonly property color elevated:      mode === "light" ? "#dcdce4" : "#2a2a38"  // badges, nav-active, chip fill
    readonly property color inset:         mode === "light" ? "#eaeaef" : "#0d0d12"  // image wells behind posters
    readonly property color border:        mode === "light" ? "#d3d3dc" : "#26262f"
    readonly property color borderStrong:  mode === "light" ? "#b8b8c4" : "#3a3a48"
    readonly property color textStrong:    mode === "light" ? "#0b0b12" : "#ffffff"
    readonly property color text:          mode === "light" ? "#1d1d26" : "#e6e6ee"
    readonly property color textDim:       mode === "light" ? "#4a4a56" : "#c0c0cc"
    readonly property color textMuted:     mode === "light" ? "#73737e" : "#8a8a96"

    // ---- brand accent (derived from accentBase so a custom colour restyles all).
    // Hover/soft variants shift toward or away from the base depending on mode so
    // they stay readable on both dark and light surfaces; accentFill is a
    // translucent wash that works over either. ----
    readonly property color accent:        accentBase
    readonly property color accentHover:   mode === "light" ? Qt.darker(accentBase, 1.12)
                                                            : Qt.lighter(accentBase, 1.18)
    readonly property color accentText:    "#ffffff"
    readonly property color accentSoft:    mode === "light" ? Qt.darker(accentBase, 1.1)
                                                            : Qt.lighter(accentBase, 1.5)
    readonly property color accentFill:    Qt.rgba(accentBase.r, accentBase.g, accentBase.b, 0.22)

    // ---- status ----                             light        dark
    readonly property color success:       mode === "light" ? "#1f9d41" : "#3ecf5b"
    readonly property color successFill:   mode === "light" ? "#dcf3e1" : "#173a22"
    readonly property color successText:   mode === "light" ? "#1a6f30" : "#8fe6a3"
    readonly property color warning:       mode === "light" ? "#b9821b" : "#e0b341"
    readonly property color warnFill:      mode === "light" ? "#f6ecd3" : "#3a3320"
    readonly property color warnText:      mode === "light" ? "#8a6410" : "#ffd98f"
    readonly property color danger:        mode === "light" ? "#cf3b3b" : "#e05a5a"
    readonly property color dangerFill:    mode === "light" ? "#f9e0e0" : "#5a1f1f"
    readonly property color dangerText:    mode === "light" ? "#9c1f1f" : "#ffd9d9"

    // ---- misc ----
    readonly property color overlay:       mode === "light" ? "#99000000" : "#cc000000"  // poster badge scrim
    readonly property color shadow:        "#000000"

    // ---- radii ----
    readonly property int radiusSm:  4
    readonly property int radius:    8
    readonly property int radiusLg:  12
    readonly property int radiusPill: 999

    // ---- form controls (one source of truth for size/shape so buttons,
    // combo boxes, text fields and spin boxes stay consistent). Height and font
    // track uiScale so larger text fits; padding tracks density. ----
    readonly property int controlHeight:  Math.round(30 * uiScale)  // buttons, combos, fields, spinboxes
    readonly property int controlRadius:  4                         // subtle rounding, not pill-like
    readonly property int controlPadding: Math.round(10 * density)  // horizontal text inset
    readonly property int controlFont:    fontMd                    // == fontMd

    // ---- spacing (scales with density) ----
    readonly property int spacingXs: Math.round(4 * density)
    readonly property int spacingSm: Math.round(8 * density)
    readonly property int spacing:   Math.round(12 * density)
    readonly property int spacingLg: Math.round(16 * density)
    readonly property int spacingXl: Math.round(24 * density)

    // ---- type scale (pixel sizes; scales with uiScale) ----
    readonly property int fontXs:   Math.round(11 * uiScale)
    readonly property int fontSm:   Math.round(12 * uiScale)
    readonly property int fontMd:   Math.round(13 * uiScale)
    readonly property int fontBase: Math.round(14 * uiScale)
    readonly property int fontLg:   Math.round(16 * uiScale)
    readonly property int fontXl:   Math.round(18 * uiScale)
    readonly property int fontXxl:  Math.round(20 * uiScale)
    readonly property int fontHero: Math.round(24 * uiScale)

    // ---- motion ----
    readonly property int durFast: 120   // hover / color state changes
    readonly property int durBase: 200   // most transitions
    readonly property int durSlow: 320   // page changes, entrances

    // Easing curves (Easing enum values; used as `easing.type: Theme.easeStandard`).
    readonly property int easeStandard: Easing.OutCubic   // general purpose
    readonly property int easeEmphasis: Easing.OutBack    // playful lift/scale
    readonly property int easeInOut:    Easing.InOutQuad  // symmetric

    // ---- poster grids (one source of truth for GridView cell size so every
    // poster grid — anime/manga libraries, search, split media grids — resizes
    // together when posterScale changes). The 180×290 base is the shipped cell;
    // AnimeCard fills the cell minus a fixed gutter, and its poster fills the
    // remaining height so posters scale proportionally. ----
    readonly property int posterCellWidth:  Math.round(180 * posterScale)
    readonly property int posterCellHeight: Math.round(290 * posterScale)

    // ---- effect tuning ----
    readonly property real cardLift:     1.03   // hover scale for poster cards
    readonly property real shadowBlur:   0.55   // MultiEffect blur (0..1)
    readonly property real shadowBlurHi: 0.85   // hovered elevation
}
