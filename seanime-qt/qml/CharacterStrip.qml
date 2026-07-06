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
        color: "#ffffff"
        font.pixelSize: 16
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
        delegate: CharacterCard { width: 96; height: 180 }
    }
}
