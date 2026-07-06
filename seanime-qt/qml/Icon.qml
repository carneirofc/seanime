import QtQuick

// A single icon glyph from the Tabler icon font (see Icons.qml). Because it's a
// Text element, it tints with a plain `color:` binding and scales with
// `size`/`font.pixelSize` just like text -- the React-Icons-style ergonomics for
// QML:  Icon { name: "search"; color: Theme.textDim }
//
// Names match tabler.io/icons; add new glyphs to the Icons singleton's map.
Text {
    id: root

    // Tabler icon name, e.g. "home", "arrow-left", "chevron-down".
    property string name: ""
    // Convenience alias for the glyph size (maps to font.pixelSize).
    property alias size: root.font.pixelSize

    text: Icons.glyph(name)
    font.family: Icons.family
    font.pixelSize: Theme.fontBase
    color: Theme.text

    horizontalAlignment: Text.AlignHCenter
    verticalAlignment: Text.AlignVCenter
    // Keep glyphs crisp at small sizes; the native rasterizer hints better than
    // distance-field text for icon fonts.
    renderType: Text.NativeRendering
}
