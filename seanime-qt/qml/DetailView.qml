import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    signal back()

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // Top bar with a back button.
        RowLayout {
            Layout.fillWidth: true
            Layout.margins: 12
            spacing: 10
            Button {
                text: "← Back"
                onClicked: root.back()
            }
            Label {
                Layout.fillWidth: true
                text: app.detailTitle
                color: "#ffffff"
                font.pixelSize: 20
                font.bold: true
                elide: Text.ElideRight
            }
        }

        // Header: poster + synopsis.
        RowLayout {
            Layout.fillWidth: true
            Layout.leftMargin: 12
            Layout.rightMargin: 12
            Layout.bottomMargin: 12
            spacing: 16

            Rectangle {
                Layout.preferredWidth: 180
                Layout.preferredHeight: 260
                radius: 8
                clip: true
                color: "#1a1a22"
                Image {
                    anchors.fill: parent
                    source: app.detailPoster
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                }
            }

            ScrollView {
                Layout.fillWidth: true
                Layout.preferredHeight: 260
                clip: true
                Label {
                    width: root.width - 180 - 44
                    text: app.detailSynopsis || "No synopsis."
                    color: "#c0c0cc"
                    font.pixelSize: 14
                    wrapMode: Text.WordWrap
                }
            }
        }

        Rectangle { Layout.fillWidth: true; height: 1; color: "#26262f" }

        Label {
            Layout.leftMargin: 12
            Layout.topMargin: 10
            text: "Episodes (" + episodeList.count + ")"
            color: "#e6e6ee"
            font.pixelSize: 15
            font.bold: true
        }

        ListView {
            id: episodeList
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.margins: 12
            clip: true
            spacing: 8
            model: app.episodeModel
            ScrollBar.vertical: ScrollBar {}
            delegate: EpisodeDelegate { width: episodeList.width }
        }
    }
}
