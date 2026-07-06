import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    signal back()

    ListEntryEditor {
        id: listEditor
        episodeCount: app.detailEpisodeCount
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // Fixed top bar with a back button.
        RowLayout {
            Layout.fillWidth: true
            Layout.margins: 12
            spacing: 10
            AppButton {
                objectName: "detailBackButton"
                text: "← Back"
                onClicked: root.back()
            }
            Label {
                objectName: "detailTitleLabel"
                Layout.fillWidth: true
                text: app.detailTitle
                color: Theme.textStrong
                font.pixelSize: Theme.fontXxl
                font.bold: true
                elide: Text.ElideRight
            }
        }

        ScrollView {
            id: scroll
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentWidth: availableWidth
            clip: true

            ColumnLayout {
                width: scroll.availableWidth
                spacing: 14

                DetailHeader {
                    Layout.fillWidth: true
                    onEditListRequested: listEditor.openFor()
                }

                Rectangle { Layout.fillWidth: true; Layout.leftMargin: 12; Layout.rightMargin: 12; Layout.preferredHeight: 1; color: Theme.border }

                // Episodes.
                Label {
                    Layout.leftMargin: 12
                    text: "Episodes (" + episodeRepeater.count + ")"
                    color: Theme.text
                    font.pixelSize: Theme.fontLg
                    font.bold: true
                    visible: episodeRepeater.count > 0
                }

                ColumnLayout {
                    Layout.fillWidth: true
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    spacing: 8
                    Repeater {
                        id: episodeRepeater
                        model: app.episodeModel
                        delegate: EpisodeDelegate {
                            Layout.fillWidth: true
                            Layout.preferredHeight: 84
                        }
                    }
                }

                // Related media, recommendations, characters.
                MediaCarousel {
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    title: "Relations"
                    model: app.relationsModel
                }
                MediaCarousel {
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    title: "Recommendations"
                    model: app.recommendationsModel
                }

                CharacterStrip {
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    model: app.characterModel
                }

                Item { Layout.preferredHeight: 12 }
            }
        }
    }
}
