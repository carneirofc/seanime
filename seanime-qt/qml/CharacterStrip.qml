import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// A titled horizontal strip of character portraits (hidden while empty).
ColumnLayout {
    id: strip
    property alias model: row.model
    property alias count: row.count

    spacing: 6
    visible: row.count > 0
    Layout.fillWidth: true

    Label {
        text: "Characters"
        color: Theme.textStrong
        font.pixelSize: Theme.fontLg
        font.bold: true
    }
    ListView {
        id: row
        Layout.fillWidth: true
        Layout.preferredHeight: 190
        orientation: ListView.Horizontal
        spacing: 12
        clip: true
        ScrollBar.horizontal: ScrollBar { policy: ScrollBar.AsNeeded }

        // Staggered fade-in as portraits populate.
        populate: Transition {
            SequentialAnimation {
                PauseAnimation { duration: Math.min(ViewTransition.index, 10) * 25 }
                NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Theme.durSlow; easing.type: Theme.easeStandard }
            }
        }
        add: Transition {
            NumberAnimation { properties: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
        }

        delegate: CharacterCard { width: 96; height: 180 }
    }
}
