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
            AppButton {
                objectName: "mangaBackButton"
                text: "← Back"
                onClicked: root.back()
            }
            Label {
                objectName: "mangaTitleLabel"
                Layout.fillWidth: true
                text: app.mangaTitle
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

                // ---- hero ----
                Rectangle {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 160
                    clip: true
                    color: Theme.surfaceAlt
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
                            GradientStop { position: 1.0; color: Theme.bg }
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
                        radius: Theme.radius
                        clip: true
                        color: Theme.surface
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
                                textColor: Theme.warnText
                                fillColor: Theme.warnFill
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
                            color: Theme.textDim
                            font.pixelSize: Theme.fontMd
                        }

                        Label {
                            Layout.fillWidth: true
                            text: app.mangaSynopsis || "No synopsis."
                            color: Theme.textDim
                            font.pixelSize: Theme.fontBase
                            wrapMode: Text.WordWrap
                        }
                    }
                }

                Rectangle { Layout.fillWidth: true; Layout.leftMargin: 12; Layout.rightMargin: 12; Layout.preferredHeight: 1; color: Theme.border }

                // ---- chapter source selector ----
                RowLayout {
                    Layout.fillWidth: true
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    spacing: 10
                    Label {
                        text: "Source"
                        color: Theme.textMuted
                        font.pixelSize: Theme.fontMd
                    }
                    AppComboBox {
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
                        color: Theme.text
                        font.pixelSize: Theme.fontLg
                        font.bold: true
                    }
                }

                // ---- chapter list ----
                Label {
                    Layout.leftMargin: 12
                    Layout.rightMargin: 12
                    visible: chapterList.count === 0
                    color: Theme.textMuted
                    font.pixelSize: Theme.fontMd
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
