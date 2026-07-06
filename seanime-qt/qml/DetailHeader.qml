import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// The detail-page "hero": banner, poster, metadata/genre chips, list-entry
// status + edit action, and synopsis. Reads the app.detail* properties; asks
// the parent to open the list editor via editListRequested().
ColumnLayout {
    id: header
    signal editListRequested()
    spacing: 14

    // Banner behind the header.
    Rectangle {
        Layout.fillWidth: true
        Layout.preferredHeight: 160
        clip: true
        color: "#14141c"
        visible: app.detailBanner.length > 0
        Image {
            anchors.fill: parent
            source: app.detailBanner
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
        }
        // Fade to background at the bottom edge.
        Rectangle {
            anchors.fill: parent
            gradient: Gradient {
                GradientStop { position: 0.4; color: "transparent" }
                GradientStop { position: 1.0; color: "#0e0e12" }
            }
        }
    }

    // Poster + title/metadata/actions/synopsis.
    RowLayout {
        Layout.fillWidth: true
        Layout.leftMargin: 12
        Layout.rightMargin: 12
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

        ColumnLayout {
            Layout.fillWidth: true
            Layout.alignment: Qt.AlignTop
            spacing: 10

            // Metadata chips.
            Flow {
                Layout.fillWidth: true
                spacing: 6
                Chip {
                    visible: app.detailScore > 0
                    text: "★ " + app.detailScore
                    textColor: "#ffd98f"
                    fillColor: "#3a3320"
                }
                Chip { visible: app.detailFormat.length > 0; text: app.detailFormat }
                Chip {
                    visible: app.detailEpisodeCount > 0
                    text: app.detailEpisodeCount + " eps"
                }
                Chip { visible: app.detailSeason.length > 0; text: app.detailSeason }
                Chip {
                    visible: app.detailDuration > 0
                    text: app.detailDuration + "m"
                }
                Chip { visible: app.detailStatus.length > 0; text: app.detailStatus }
                Chip {
                    visible: app.detailNextAiring.length > 0
                    text: app.detailNextAiring
                    textColor: "#8fd7ff"
                    fillColor: "#1f2f3a"
                }
            }

            // Genres.
            Flow {
                Layout.fillWidth: true
                spacing: 6
                visible: app.detailGenres.length > 0
                Repeater {
                    model: app.detailGenres
                    delegate: Chip { text: modelData }
                }
            }

            // List-entry status + edit action.
            RowLayout {
                Layout.fillWidth: true
                spacing: 10
                Label {
                    text: app.detailListStatus.length > 0
                          ? "On list: " + app.detailListStatus
                            + " · " + app.detailListProgress
                            + (app.detailEpisodeCount > 0 ? "/" + app.detailEpisodeCount : "")
                          : "Not in your list"
                    color: "#c0c0cc"
                    font.pixelSize: 13
                }
                Item { Layout.fillWidth: true }
                Button {
                    objectName: "editListButton"
                    text: "Edit list"
                    onClicked: header.editListRequested()
                }
            }

            Label {
                Layout.fillWidth: true
                text: app.detailSynopsis || "No synopsis."
                color: "#c0c0cc"
                font.pixelSize: 14
                wrapMode: Text.WordWrap
            }
        }
    }
}
