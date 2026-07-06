import QtQuick
import QtQuick.Controls

Item {
    id: card

    // Addressable by the agent harness: each card carries its media id so a
    // specific poster can be located and clicked (e.g. "card_21").
    objectName: "card_" + mediaId

    // Exposed so the enclosing GridView can activate the current card by keyboard.
    property int cardMediaId: mediaId

    // Exposed so the delegate user can react to clicks.
    signal activated()
    function activate() { card.activated() }

    // Accessibility: each poster is announced as a button named after the title.
    // Arrow-key focus and Enter activation are handled by the GridView (the focus
    // scope), so the card itself is not individually tab-focusable.
    Accessible.role: Accessible.Button
    Accessible.name: title
    Accessible.description: "Open details for " + title
    Accessible.focusable: true
    Accessible.onPressAction: card.activate()

    Rectangle {
        anchors.fill: parent
        radius: 8
        color: hover.hovered ? "#22222c" : "#1a1a22"
        border.color: hover.hovered ? "#3a3a48" : "transparent"
        border.width: 1

        Column {
            anchors.fill: parent
            anchors.margins: 8
            spacing: 6

            Rectangle {
                width: parent.width
                height: 210
                radius: 6
                clip: true
                color: "#0e0e12"

                Image {
                    anchors.fill: parent
                    source: posterUrl
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                }

                // Progress / episode-count badge.
                Rectangle {
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: 4
                    height: 20
                    width: badge.implicitWidth + 12
                    radius: 4
                    color: "#cc000000"
                    Label {
                        id: badge
                        anchors.centerIn: parent
                        text: progress + (episodeCount > 0 ? "/" + episodeCount : "")
                        color: "#ffffff"
                        font.pixelSize: 11
                    }
                }
            }

            Label {
                width: parent.width
                text: title
                color: "#e6e6ee"
                font.pixelSize: 13
                elide: Text.ElideRight
                maximumLineCount: 2
                wrapMode: Text.WordWrap
            }
        }
    }

    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
    TapHandler { onTapped: card.activate() }
}
