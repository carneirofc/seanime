import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Appearance pane for the settings screen: client-local UI controls (font size,
// density, theme, accent colour). Unlike the other panes there is no Save button
// — every change is applied and persisted immediately through the app.setUi*
// slots, and reflected live by the Theme singleton.
//
// Each control seeds itself from the current app pref on completion and re-syncs
// via a Connections on app.uiPrefsChanged (so "Reset to defaults" — an external
// change — moves them too), while user edits write straight through app.setUi*.
Flickable {
    id: pane
    objectName: "appearancePane"
    contentHeight: col.implicitHeight + 2 * Theme.spacingLg
    clip: true
    ScrollBar.vertical: ScrollBar {}

    readonly property var densityOptions: [
        { label: "Compact", value: 0.85 },
        { label: "Comfortable", value: 1.0 },
        { label: "Spacious", value: 1.2 }
    ]
    readonly property var themeOptions: [
        { label: "Dark", value: "dark" },
        { label: "Light", value: "light" }
    ]
    readonly property var splitOverrideOptions: [
        { label: "Follow server setting", value: "server" },
        { label: "Always split", value: "on" },
        { label: "Never split", value: "off" }
    ]
    // Preset accent swatches (the first is the shipped default).
    readonly property var accentSwatches: [
        "#6152df", "#41b8e0", "#3ecf5b", "#e0b341",
        "#e08541", "#e05a5a", "#d764c9"
    ]

    // Index of the option whose numeric value is closest to v — avoids exact
    // float-equality pitfalls when matching a stored density factor.
    function nearestIndex(model, v) {
        var best = 0, bestDist = Infinity
        for (var i = 0; i < model.length; i++) {
            var d = Math.abs(model[i].value - v)
            if (d < bestDist) { bestDist = d; best = i }
        }
        return best
    }

    ColumnLayout {
        id: col
        x: Theme.spacingLg
        y: Theme.spacingLg
        width: pane.width - 2 * Theme.spacingLg
        spacing: Theme.spacingLg

        Label {
            text: "These settings apply instantly and are stored on this computer."
            color: Theme.textMuted
            font.pixelSize: Theme.fontSm
            Layout.fillWidth: true
            wrapMode: Text.WordWrap
        }

        // ---- font size ----
        ColumnLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingXs
            Label { text: "Font size"; color: Theme.text; font.pixelSize: Theme.fontBase }
            Label {
                text: "Scales all text and controls (80–150%)."
                color: Theme.textMuted; font.pixelSize: Theme.fontSm
            }
            AppSpinBox {
                id: scaleBox
                objectName: "uiScaleSpin"
                from: 80; to: 150; stepSize: 5; editable: true
                Layout.preferredWidth: 130
                textFromValue: function(value, locale) { return value + "%" }
                valueFromText: function(text, locale) { return parseInt(text) || 100 }
                Component.onCompleted: value = Math.round(app.uiScale * 100)
                onValueModified: app.setUiScale(value / 100)
                Connections {
                    target: app
                    function onUiPrefsChanged() {
                        scaleBox.value = Math.round(app.uiScale * 100)
                    }
                }
            }
        }

        // ---- poster size ----
        ColumnLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingXs
            Label { text: "Poster size"; color: Theme.text; font.pixelSize: Theme.fontBase }
            Label {
                text: "Scales poster cards across the library, manga and search grids (70–140%)."
                color: Theme.textMuted; font.pixelSize: Theme.fontSm
                Layout.fillWidth: true
                wrapMode: Text.WordWrap
            }
            RowLayout {
                Layout.fillWidth: true
                spacing: Theme.spacingSm
                Slider {
                    id: posterSlider
                    objectName: "uiPosterScaleSlider"
                    Layout.preferredWidth: 220
                    from: 0.7; to: 1.4; stepSize: 0.05
                    snapMode: Slider.SnapAlways
                    // Seed from the current pref; user drags write straight through,
                    // and the snap step keeps a single drag to a handful of distinct
                    // (persisted) values rather than one per pixel.
                    Component.onCompleted: value = app.uiPosterScale
                    onMoved: app.setUiPosterScale(value)
                    Connections {
                        target: app
                        function onUiPrefsChanged() {
                            posterSlider.value = app.uiPosterScale
                        }
                    }
                }
                Label {
                    objectName: "uiPosterScaleValue"
                    text: Math.round(posterSlider.value * 100) + "%"
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontSm
                    Layout.preferredWidth: 44
                }
            }
        }

        // ---- adult content split ----
        ColumnLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingXs
            Label { text: "Split adult content"; color: Theme.text; font.pixelSize: Theme.fontBase }
            Label {
                text: "Keep adult titles in their own sections across the library, "
                      + "search, manga and Discover previews. \"Follow server setting\" "
                      + "uses your Seanime server's preference."
                color: Theme.textMuted; font.pixelSize: Theme.fontSm
                Layout.fillWidth: true
                wrapMode: Text.WordWrap
            }
            AppComboBox {
                id: splitCombo
                objectName: "splitAdultOverrideCombo"
                Layout.preferredWidth: 220
                model: pane.splitOverrideOptions
                textRole: "label"; valueRole: "value"
                Component.onCompleted: currentIndex = splitCombo.indexOfValue(app.splitAdultOverride)
                onActivated: app.setSplitAdultOverride(currentValue)
                Connections {
                    target: app
                    function onAdultSplitChanged() {
                        splitCombo.currentIndex = splitCombo.indexOfValue(app.splitAdultOverride)
                    }
                }
            }
        }

        // ---- density ----
        ColumnLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingXs
            Label { text: "Density"; color: Theme.text; font.pixelSize: Theme.fontBase }
            Label {
                text: "Spacing and padding between elements."
                color: Theme.textMuted; font.pixelSize: Theme.fontSm
            }
            AppComboBox {
                id: densityCombo
                objectName: "uiDensityCombo"
                Layout.preferredWidth: 220
                model: pane.densityOptions
                textRole: "label"; valueRole: "value"
                Component.onCompleted:
                    currentIndex = pane.nearestIndex(pane.densityOptions, app.uiDensity)
                onActivated: app.setUiDensity(currentValue)
                Connections {
                    target: app
                    function onUiPrefsChanged() {
                        densityCombo.currentIndex =
                            pane.nearestIndex(pane.densityOptions, app.uiDensity)
                    }
                }
            }
        }

        // ---- theme ----
        ColumnLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingXs
            Label { text: "Theme"; color: Theme.text; font.pixelSize: Theme.fontBase }
            AppComboBox {
                id: themeCombo
                objectName: "uiThemeCombo"
                Layout.preferredWidth: 220
                model: pane.themeOptions
                textRole: "label"; valueRole: "value"
                Component.onCompleted: currentIndex = themeCombo.indexOfValue(app.uiThemeMode)
                onActivated: app.setUiThemeMode(currentValue)
                Connections {
                    target: app
                    function onUiPrefsChanged() {
                        themeCombo.currentIndex = themeCombo.indexOfValue(app.uiThemeMode)
                    }
                }
            }
        }

        // ---- accent colour ----
        ColumnLayout {
            Layout.fillWidth: true
            spacing: Theme.spacingSm
            Label { text: "Accent color"; color: Theme.text; font.pixelSize: Theme.fontBase }

            RowLayout {
                spacing: Theme.spacingSm
                Repeater {
                    model: pane.accentSwatches
                    delegate: Rectangle {
                        id: swatch
                        required property string modelData
                        objectName: "accentSwatch_" + modelData
                        readonly property bool selected:
                            app.uiAccent.toLowerCase() === modelData.toLowerCase()
                        width: 30; height: 30; radius: Theme.radiusSm
                        color: modelData
                        border.width: selected ? 3 : 1
                        border.color: selected ? Theme.textStrong : Theme.border
                        HoverHandler { cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: app.setUiAccent(swatch.modelData) }
                    }
                }
            }

            RowLayout {
                spacing: Theme.spacingSm
                Label { text: "Custom hex"; color: Theme.textMuted; font.pixelSize: Theme.fontSm }
                AppTextField {
                    id: hexField
                    objectName: "accentHexField"
                    Layout.preferredWidth: 120
                    placeholderText: "#6152df"
                    text: app.uiAccent
                    onAccepted: app.setUiAccent(text)
                }
                AppButton {
                    objectName: "accentApplyButton"
                    text: "Apply"
                    onClicked: app.setUiAccent(hexField.text)
                }
            }
        }

        // ---- live preview ----
        Rectangle {
            Layout.fillWidth: true
            Layout.maximumWidth: 460
            Layout.topMargin: Theme.spacingSm
            radius: Theme.radius
            color: Theme.surface
            border.width: 1
            border.color: Theme.border
            implicitHeight: previewCol.implicitHeight + 2 * Theme.spacing

            ColumnLayout {
                id: previewCol
                x: Theme.spacing
                y: Theme.spacing
                width: parent.width - 2 * Theme.spacing
                spacing: Theme.spacingSm

                Label {
                    text: "Preview"
                    color: Theme.textStrong
                    font.pixelSize: Theme.fontXl
                    font.bold: true
                }
                Label {
                    text: "The quick brown fox jumps over the lazy dog."
                    color: Theme.text
                    font.pixelSize: Theme.fontBase
                    Layout.fillWidth: true
                    wrapMode: Text.WordWrap
                }
                Label {
                    text: "Secondary, muted caption text."
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontSm
                }
                RowLayout {
                    spacing: Theme.spacingSm
                    // `checked` (without `checkable`) renders the accent fill, so
                    // the primary button previews the chosen accent colour live.
                    AppButton { text: "Primary"; checked: true }
                    AppButton { text: "Secondary" }
                }

                // ---- poster preview ----
                // A stand-in poster card sized from the same Theme.posterCell*
                // values the real grids use, so dragging the "Poster size" slider
                // above grows and shrinks it live. Built inline rather than reusing
                // AnimeCard (which needs model roles) to keep the preview standalone.
                Label {
                    text: "Poster (" + Math.round(posterSlider.value * 100) + "%)"
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontSm
                    Layout.topMargin: Theme.spacingSm
                }
                Rectangle {
                    objectName: "posterPreviewCard"
                    Layout.preferredWidth: Theme.posterCellWidth
                    Layout.preferredHeight: Theme.posterCellHeight
                    radius: Theme.radius
                    color: Theme.surfaceHover
                    border.width: 1
                    border.color: Theme.border
                    Behavior on Layout.preferredWidth { NumberAnimation { duration: Theme.durFast } }
                    Behavior on Layout.preferredHeight { NumberAnimation { duration: Theme.durFast } }

                    Item {
                        anchors.fill: parent
                        anchors.margins: 8

                        // Poster well fills the space above the title, mirroring
                        // AnimeCard's layout so the proportions match the real card.
                        Rectangle {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.bottom: posterTitle.top
                            anchors.bottomMargin: 6
                            radius: 6
                            color: Theme.inset

                            Icon {
                                anchors.centerIn: parent
                                name: "photo"
                                size: 28
                                color: Theme.textMuted
                            }
                            // Accent-tinted progress badge, echoing the real card.
                            Rectangle {
                                anchors.right: parent.right
                                anchors.top: parent.top
                                anchors.margins: 4
                                height: 20
                                width: previewBadge.implicitWidth + 12
                                radius: Theme.radiusSm
                                color: Theme.overlay
                                Label {
                                    id: previewBadge
                                    anchors.centerIn: parent
                                    text: "3/12"
                                    color: Theme.textStrong
                                    font.pixelSize: Theme.fontXs
                                }
                            }
                        }
                        Label {
                            id: posterTitle
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            text: "Sample Title"
                            color: Theme.text
                            font.pixelSize: Theme.fontMd
                            elide: Text.ElideRight
                            maximumLineCount: 2
                            wrapMode: Text.WordWrap
                        }
                    }
                }
            }
        }
    }
}
