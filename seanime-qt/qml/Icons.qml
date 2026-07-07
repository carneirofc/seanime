pragma Singleton
import QtQuick

// Icon glyph registry, backed by the Tabler Icons webfont (MIT-licensed,
// 5000+ outline glyphs -- https://tabler.io/icons). This replaces the emoji and
// stray Unicode symbols the UI used as icons, which rendered as multicolour OS
// glyphs that could not be tinted and looked different on every platform.
//
// A single .ttf is loaded once here and every icon is drawn as a `Text` element
// (see Icon.qml), so an icon recolours for free with a plain `color:` binding
// against the Theme tokens -- exactly like text.
//
// Usage:  Icon { name: "search" }   // names match tabler.io/icons
// Add new glyphs to the map below: copy the codepoint from the font CSS at
// tabler.io/icons and add a "name": "\uXXXX" entry. Registered via qmldir.
QtObject {
    id: root

    // Loaded once for the whole process. Qt.resolvedUrl keeps the path relative
    // to this file, so it works from source and from an installed wheel alike.
    property FontLoader _loader: FontLoader {
        source: Qt.resolvedUrl("fonts/tabler-icons.ttf")
    }

    // Font family to assign to a Text's font.family. Empty until the FontLoader
    // finishes; bindings re-evaluate when it becomes Ready.
    readonly property string family: _loader.status === FontLoader.Ready ? _loader.name : ""
    readonly property bool ready: _loader.status === FontLoader.Ready

    // Semantic name -> glyph character. Keys mirror the Tabler icon names. Only
    // the outline set ships in the free webfont (no "-filled" variants); an
    // outline star tinted with an accent reads cleanly as a rating star.
    readonly property var glyphs: ({
        // navigation / chrome
        "home":                    "\ueac1",
        "books":                   "\ueff2",
        "book":                    "\uea39",
        "compass":                 "\uea79",
        "search":                  "\ueb1c",
        "user":                    "\ueb4d",
        "settings":                "\ueb20",
        "key":                     "\ueac7",
        "logout":                  "\ueba8",
        "login":                   "\ueba7",
        "list":                    "\ueb6b",
        "layout-grid":             "\uedba",
        "filter":                  "\ueaa5",
        "adjustments-horizontal":  "\uec38",
        "refresh":                 "\ueb13",
        "calendar":                "\uea53",
        "clock":                   "\uea70",
        // arrows / carets
        "arrow-left":              "\uea19",
        "chevron-down":            "\uea5f",
        "chevron-up":              "\uea62",
        "chevron-left":            "\uea60",
        "chevron-right":           "\uea61",
        // actions / status
        "x":                       "\ueb55",
        "minus":                   "\ueaf2",
        "plus":                    "\ueb0b",
        "check":                   "\uea5e",
        "circle-check":            "\uea67",
        "star":                    "\ueb2e",
        "heart":                   "\ueabe",
        "bookmark":                "\uea3a",
        "dots-vertical":           "\uea94",
        "eye":                     "\uea9a",
        "eye-off":                 "\uecf0",
        "lock":                    "\ueae2",
        "photo":                   "\ueb0a",
        "download":                "\uea96",
        "player-play":             "\ued46",
        // badges
        "rating-18-plus":          "\uf269"
    })

    // Glyph char for a name, or "" (draws nothing) if unknown, so a typo shows a
    // blank rather than a tofu box. Warns to the log to surface missing names.
    function glyph(name) {
        if (!name)
            return "";  // no icon requested (e.g. an optional/hidden Icon) -- quiet
        var g = glyphs[name];
        if (g === undefined) {
            console.warn("Icons: unknown icon name '" + name + "'");
            return "";
        }
        return g;
    }
}
