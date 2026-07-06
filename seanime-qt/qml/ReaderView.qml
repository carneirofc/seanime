import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Long-strip manga reader: chapter pages stacked vertically in a single
// scrollable list. Marks the chapter read once the reader reaches the bottom
// (with an explicit button as a fallback). Reads app.pageModel / app.reader*.
Item {
    id: root
    signal back()

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // Top toolbar.
        RowLayout {
            Layout.fillWidth: true
            Layout.margins: 12
            spacing: 10
            Button {
                objectName: "readerBackButton"
                text: "← Back"
                onClicked: root.back()
            }
            Label {
                objectName: "readerTitleLabel"
                Layout.fillWidth: true
                text: app.readerChapterTitle
                color: "#ffffff"
                font.pixelSize: 17
                font.bold: true
                elide: Text.ElideRight
            }
            BusyIndicator {
                running: app.readerLoading
                visible: app.readerLoading
                implicitWidth: 22
                implicitHeight: 22
            }
            Button {
                objectName: "markReadButton"
                text: "Mark read"
                onClicked: app.markCurrentChapterRead()
            }
        }

        // Page strip.
        ListView {
            id: pageList
            objectName: "readerList"
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            model: app.pageModel
            spacing: 2
            cacheBuffer: 4000

            ScrollBar.vertical: ScrollBar {}

            // Mark the chapter read when the reader reaches the last page.
            onAtYEndChanged: {
                if (atYEnd && count > 0)
                    app.markCurrentChapterRead()
            }

            delegate: Item {
                width: pageList.width
                // Height derives from the image's aspect ratio once it loads;
                // fall back to a square-ish placeholder while loading.
                height: pageImage.implicitWidth > 0
                        ? width * pageImage.implicitHeight / pageImage.implicitWidth
                        : width

                Image {
                    id: pageImage
                    anchors.fill: parent
                    source: imageUrl
                    fillMode: Image.PreserveAspectFit
                    asynchronous: true
                    cache: false

                    // Simple loading / error affordance behind the image.
                    Rectangle {
                        anchors.centerIn: parent
                        width: parent.width * 0.6
                        height: 40
                        radius: 6
                        color: "#1a1a22"
                        visible: pageImage.status !== Image.Ready
                        Label {
                            anchors.centerIn: parent
                            color: "#8a8a96"
                            font.pixelSize: 12
                            text: pageImage.status === Image.Error
                                    ? "Failed to load page " + (pageIndex + 1)
                                    : "Loading page " + (pageIndex + 1) + "…"
                        }
                    }
                }
            }

            // Empty state.
            Label {
                anchors.centerIn: parent
                visible: pageList.count === 0 && !app.readerLoading
                color: "#8a8a96"
                font.pixelSize: 14
                text: "No pages to display."
            }
        }
    }
}
