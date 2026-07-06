import QtQuick
import QtQuick.Controls
import QtQuick.Effects

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

    // Lift the poster on hover; keep the raised card above its neighbours.
    scale: hover.hovered ? Theme.cardLift : 1.0
    z: hover.hovered ? 1 : 0
    Behavior on scale { NumberAnimation { duration: Theme.durBase; easing.type: Theme.easeEmphasis } }

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
        radius: Theme.radius
        color: hover.hovered ? Theme.surfaceHover : Theme.surface
        border.color: hover.hovered ? Theme.borderStrong : "transparent"
        border.width: 1

        Behavior on color { ColorAnimation { duration: Theme.durFast } }
        Behavior on border.color { ColorAnimation { duration: Theme.durFast } }

        // Soft elevation shadow that deepens on hover (GPU MultiEffect, Qt 6.5+).
        layer.enabled: true
        layer.effect: MultiEffect {
            shadowEnabled: true
            shadowColor: Theme.shadow
            shadowVerticalOffset: hover.hovered ? 8 : 3
            shadowBlur: hover.hovered ? Theme.shadowBlurHi : Theme.shadowBlur
            autoPaddingEnabled: true
            Behavior on shadowBlur { NumberAnimation { duration: Theme.durBase } }
            Behavior on shadowVerticalOffset { NumberAnimation { duration: Theme.durBase } }
        }

        Column {
            anchors.fill: parent
            anchors.margins: 8
            spacing: 6

            Rectangle {
                width: parent.width
                height: 210
                radius: 6
                clip: true
                color: Theme.inset

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
                    radius: Theme.radiusSm
                    color: Theme.overlay
                    Label {
                        id: badge
                        anchors.centerIn: parent
                        text: progress + (episodeCount > 0 ? "/" + episodeCount : "")
                        color: Theme.textStrong
                        font.pixelSize: Theme.fontXs
                    }
                }
            }

            Label {
                width: parent.width
                text: title
                color: Theme.text
                font.pixelSize: Theme.fontMd
                elide: Text.ElideRight
                maximumLineCount: 2
                wrapMode: Text.WordWrap
            }
        }
    }

    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
    TapHandler { onTapped: card.activate() }
}
