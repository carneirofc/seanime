import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// One chapter row in the manga detail chapter list. Tapping opens the reader.
// Model roles: chapterId, title, number, scanlator, language, read.
Rectangle {
    id: chapterRow
    objectName: "chapter_" + chapterId

    radius: Theme.radius
    color: hover.hovered ? Theme.surfaceHover : Theme.surface
    border.color: hover.hovered ? Theme.borderStrong : "transparent"
    border.width: 1

    Behavior on color { ColorAnimation { duration: Theme.durFast } }
    Behavior on border.color { ColorAnimation { duration: Theme.durFast } }

    function activate() { app.openChapter(chapterId, number, title) }

    Accessible.role: Accessible.Button
    Accessible.name: title + (read ? " (read)" : "")
    Accessible.description: "Open chapter in the reader"
    Accessible.focusable: true
    Accessible.onPressAction: chapterRow.activate()

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: 12
        anchors.rightMargin: 12
        spacing: 10

        // Read indicator.
        Rectangle {
            width: 9; height: 9; radius: 5
            color: read ? Theme.success : Theme.borderStrong
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 2
            Label {
                Layout.fillWidth: true
                text: title
                color: read ? Theme.textMuted : Theme.text
                font.pixelSize: Theme.fontMd
                elide: Text.ElideRight
            }
            Label {
                visible: scanlator.length > 0 || language.length > 0
                text: [scanlator, language].filter(function(s) { return s.length > 0 }).join(" · ")
                color: Theme.textMuted
                font.pixelSize: Theme.fontXs
            }
        }

        Row {
            visible: read
            spacing: 3
            Icon {
                name: "check"
                size: Theme.fontXs
                color: Theme.success
                anchors.verticalCenter: parent.verticalCenter
            }
            Label {
                text: "Read"
                color: Theme.success
                font.pixelSize: Theme.fontXs
                anchors.verticalCenter: parent.verticalCenter
            }
        }
    }

    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
    TapHandler { onTapped: chapterRow.activate() }
}
