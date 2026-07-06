import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Manga detail: hero (banner/poster/metadata/synopsis), a chapter-source
// selector, and the chapter list. Mirrors DetailView but reads app.manga*.
Item {
    id: root
    signal back()

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // Fixed top bar with a back button.
        RowLayout {
            Layout.fillWidth: true
            Layout.margins: 12
            spacing: 10
            Button {
                objectName: "mangaBackButton"
                text: "← Back"
                onClicked: root.back()
            }
            Label {
                objectName: "mangaTitleLabel"
                Layout.fillWidth: true
                text: app.mangaTitle
                color: "#ffffff"
                font.pixelSize: 20
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

                // ---- hero ----
                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 160
                    clip: true
                    color: "#14141c"
                    visible: app.mangaBanner.length > 0
                    Image {
                        anchors.fill: parent
                        source: app.mangaBanner
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                    }
                    Rectangle {
                        anchors.fill: parent
                        gradient: Gradient {
                            GradientStop { position: 0.4; color: "transparent" }
                            GradientStop { position: 1.0; color: "#0e0e12" }
                        }
                    }
                }

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
                            source: app.mangaPoster
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
                                visible: app.mangaScore > 0
                                text: "★ " + app.mangaScore
                                textColor: "#ffd98f"
                                fillColor: "#3a3320"
                            }
                            Chip { visible: app.mangaFormat.length > 0; text: app.mangaFormat }
                            Chip {
                                visible: app.mangaChapterCount > 0
                                text: app.mangaChapterCount + " ch"
                            }
                            Chip { visible: app.mangaStatus.length > 0; text: app.mangaStatus }
                        }

                        // Genres.
                        Flow {
                            Layout.fillWidth: true
                            spacing: 6
                            visible: app.mangaGenres.length > 0
                            Repeater {
                                model: app.mangaGenres
                                delegate: Chip { text: modelData }
                            }
                        }

                        // List-entry status.
                        Label {
                            text: app.mangaListStatus.length > 0
                                  ? "On list: " + app.mangaListStatus
                                    + " · " + app.mangaListProgress
                                    + (app.mangaChapterCount > 0 ? "/" + app.mangaChapterCount : "")
                                  : "Not in your list"
                            color: "#c0c0cc"
                            font.pixelSize: 13
                        }

                        Label {
                            Layout.fillWidth: true
                            text: app.mangaSynopsis || "No synopsis."
                            color: "#c0c0cc"
                            font.pixelSize: 14
                            wrapMode: Text.WordWrap
                        }
                    }
                }

                Rectangle { Layout.fillWidth: true; Layout.leftMargin: 12; Layout.rightMargin: 12; Layout.preferredHeight: 1; color: "#26262f" }

                // ---- chapter source selector ----
                RowLayout {
                    Layout.fillWidth: true
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    spacing: 10
                    Label {
                        text: "Source"
                        color: "#8a8a96"
                        font.pixelSize: 13
                    }
                    ComboBox {
                        id: providerCombo
                        objectName: "providerCombo"
                        Layout.preferredWidth: 220
                        model: app.mangaProviders
                        textRole: "name"
                        valueRole: "id"
                        enabled: app.mangaProviders.length > 0
                        // Reflect the controller's active provider.
                        currentIndex: indexOfValue(app.currentMangaProvider)
                        onActivated: app.setMangaProvider(currentValue)
                    }
                    Item { Layout.fillWidth: true }
                    Label {
                        text: "Chapters (" + chapterList.count + ")"
                        color: "#e6e6ee"
                        font.pixelSize: 15
                        font.bold: true
                    }
                }

                // ---- chapter list ----
                Label {
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    visible: chapterList.count === 0
                    color: "#8a8a96"
                    font.pixelSize: 13
                    text: app.mangaProviders.length === 0
                            ? "No manga provider installed. Install one in the Seanime web UI."
                            : "No chapters found for this source. Try another source above."
                }

                ColumnLayout {
                    Layout.fillWidth: true
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    spacing: 6
                    Repeater {
                        id: chapterList
                        model: app.chapterModel
                        delegate: ChapterDelegate {
                            Layout.fillWidth: true
                            Layout.preferredHeight: 52
                        }
                    }
                }

                Item { Layout.preferredHeight: 12 }
            }
        }
    }
}
