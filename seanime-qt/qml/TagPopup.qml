import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// AniList tag multi-select. Unlike the short genre list this catalog holds a few
// hundred tags grouped by category, so it's a searchable, virtualized list with
// section headers. Owns the current selection; the parent reads `selected`.
// Adult-flagged tags are only offered when the server enables adult content.
Popup {
    id: tagPopup
    objectName: "tagPopup"
    modal: true
    anchors.centerIn: Overlay.overlay
    width: 460
    height: Math.min(560, Overlay.overlay ? Overlay.overlay.height - 80 : 560)
    padding: 16
    background: Rectangle { color: Theme.surface; radius: Theme.radius; border.color: Theme.border }

    enter: Transition {
        ParallelAnimation {
            NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
            NumberAnimation { property: "scale"; from: 0.96; to: 1; duration: Theme.durBase; easing.type: Theme.easeEmphasis }
        }
    }
    exit: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Theme.durFast }
    }

    // Currently-selected tag names.
    property var selected: []
    // Live filter typed into the search box.
    property string filterText: ""

    // The tag catalog, filtered by the search box and by adult visibility, sorted
    // by category then name so the ListView sections line up.
    readonly property var visibleTags: {
        var q = filterText.trim().toLowerCase()
        var allowAdult = app.enableAdultContent
        var out = []
        var all = app.mediaTags
        for (var i = 0; i < all.length; i++) {
            var t = all[i]
            if (t.isAdult && !allowAdult)
                continue
            if (q.length > 0 && t.name.toLowerCase().indexOf(q) < 0)
                continue
            out.push(t)
        }
        out.sort(function (a, b) {
            if (a.category === b.category)
                return a.name < b.name ? -1 : (a.name > b.name ? 1 : 0)
            return a.category < b.category ? -1 : 1
        })
        return out
    }

    function toggle(name) {
        var list = selected.slice()
        var i = list.indexOf(name)
        if (i >= 0) list.splice(i, 1)
        else list.push(name)
        selected = list
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            Label {
                text: tagPopup.selected.length > 0
                      ? "Tags (" + tagPopup.selected.length + ")"
                      : "Tags"
                color: Theme.textStrong
                font.pixelSize: Theme.fontLg
                font.bold: true
            }
            Item { Layout.fillWidth: true }
            Label {
                visible: !app.enableAdultContent
                text: "Adult tags hidden"
                color: Theme.textMuted
                font.pixelSize: Theme.fontXs
            }
        }

        AppTextField {
            id: tagSearch
            objectName: "tagSearchField"
            Layout.fillWidth: true
            placeholderText: "Filter tags…"
            onTextChanged: tagPopup.filterText = text
            focus: true
        }

        ListView {
            id: tagList
            objectName: "tagList"
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            model: tagPopup.visibleTags
            spacing: 2
            ScrollBar.vertical: ScrollBar {}

            section.property: "category"
            section.criteria: ViewSection.FullString
            section.delegate: Label {
                width: ListView.view.width
                text: section
                color: Theme.accentSoft
                font.pixelSize: Theme.fontXs
                font.bold: true
                topPadding: 8
                bottomPadding: 2
            }

            delegate: ItemDelegate {
                id: row
                required property var modelData
                width: ListView.view ? ListView.view.width : 0
                height: 34
                readonly property bool picked: tagPopup.selected.indexOf(modelData.name) >= 0

                contentItem: RowLayout {
                    spacing: 8
                    Rectangle {
                        Layout.alignment: Qt.AlignVCenter
                        width: 16; height: 16; radius: Theme.radiusSm
                        color: row.picked ? Theme.accent : "transparent"
                        border.color: row.picked ? Theme.accent : Theme.borderStrong
                        border.width: 1
                        Label {
                            anchors.centerIn: parent
                            visible: row.picked
                            text: "✓"
                            color: Theme.accentText
                            font.pixelSize: Theme.fontXs
                        }
                    }
                    Label {
                        Layout.fillWidth: true
                        text: row.modelData.name
                        color: Theme.text
                        font.pixelSize: Theme.fontMd
                        elide: Text.ElideRight
                    }
                    Label {
                        visible: row.modelData.isAdult
                        text: "18+"
                        color: Theme.dangerText
                        font.pixelSize: Theme.fontXs
                        font.bold: true
                    }
                }
                onClicked: tagPopup.toggle(modelData.name)
            }

            Label {
                anchors.centerIn: parent
                visible: tagList.count === 0
                text: "No tags match."
                color: Theme.textMuted
                font.pixelSize: Theme.fontMd
            }
        }

        RowLayout {
            Layout.fillWidth: true
            AppButton { text: "Clear"; enabled: tagPopup.selected.length > 0; onClicked: tagPopup.selected = [] }
            Item { Layout.fillWidth: true }
            AppButton { text: "Done"; onClicked: tagPopup.close() }
        }
    }
}
