import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// One torrent search result row: name + metadata chips + a Download action.
// Tapping the row or the button asks to download this torrent (emits selected()).
Rectangle {
    id: row
    required property string name
    required property string formattedSize
    required property int seeders
    required property int leechers
    required property string resolution
    required property string releaseGroup
    required property bool isBatch
    required property bool isBestRelease
    required property bool confirmed
    required property string date
    signal selected()

    height: content.implicitHeight + 20
    radius: Theme.radius
    color: hover.hovered ? Theme.surfaceHover : Theme.surface
    border.width: 1
    border.color: hover.hovered ? Theme.border : "transparent"
    Behavior on color { ColorAnimation { duration: Theme.durFast } }

    Accessible.role: Accessible.ListItem
    Accessible.name: row.name

    HoverHandler { id: hover }
    TapHandler { onTapped: row.selected() }

    RowLayout {
        id: content
        anchors.fill: parent
        anchors.margins: 10
        spacing: 12

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 6
            Label {
                Layout.fillWidth: true
                text: row.name
                color: Theme.text
                font.pixelSize: Theme.fontBase
                font.bold: true
                elide: Text.ElideRight
            }
            Flow {
                Layout.fillWidth: true
                spacing: 6
                Chip {
                    visible: row.confirmed
                    text: "Confirmed"
                    icon: "circle-check"
                    textColor: Theme.successText
                    fillColor: Theme.successFill
                }
                Chip {
                    visible: row.isBestRelease
                    text: "Best"
                    textColor: Theme.warnText
                    fillColor: Theme.warnFill
                }
                Chip { visible: row.isBatch; text: "Batch" }
                Chip { visible: row.resolution.length > 0; text: row.resolution }
                Chip { visible: row.formattedSize.length > 0; text: row.formattedSize }
                Chip {
                    text: row.seeders + " S"
                    textColor: Theme.successText
                    fillColor: Theme.successFill
                }
                Chip { text: row.leechers + " L" }
                Chip { visible: row.releaseGroup.length > 0; text: row.releaseGroup }
                Chip { visible: row.date.length > 0; text: row.date }
            }
        }

        AppButton {
            text: "Download"
            iconName: "download"
            Layout.alignment: Qt.AlignVCenter
            onClicked: row.selected()
        }
    }
}
