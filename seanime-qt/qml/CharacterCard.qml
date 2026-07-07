import QtQuick
import QtQuick.Controls
import QtQuick.Effects

// A small character portrait + name + role, used in the DetailView strip.
// Clicking opens the character's AniList page in the system default browser.
Item {
    id: card

    objectName: "characterCard_" + index
    readonly property bool linkable: typeof siteUrl === "string" && siteUrl.length > 0

    function openLink() {
        if (linkable)
            Qt.openUrlExternally(siteUrl)
    }

    // Gentle lift on hover for a bit of life in the strip.
    scale: hover.hovered ? 1.04 : 1.0
    z: hover.hovered ? 1 : 0
    Behavior on scale { NumberAnimation { duration: Theme.durBase; easing.type: Theme.easeEmphasis } }

    Accessible.role: Accessible.Button
    Accessible.name: name
    Accessible.description: linkable ? "Open " + name + " on AniList in your browser" : ""
    Accessible.onPressAction: openLink()

    Column {
        anchors.fill: parent
        spacing: 4

        Rectangle {
            width: parent.width
            height: 132
            radius: Theme.radius
            clip: true
            color: Theme.inset

            // Elevation shadow (deepens on hover).
            layer.enabled: true
            layer.effect: MultiEffect {
                shadowEnabled: true
                shadowColor: Theme.shadow
                shadowVerticalOffset: hover.hovered ? 7 : 2
                shadowBlur: hover.hovered ? Theme.shadowBlurHi : Theme.shadowBlur
                autoPaddingEnabled: true
                Behavior on shadowBlur { NumberAnimation { duration: Theme.durBase } }
                Behavior on shadowVerticalOffset { NumberAnimation { duration: Theme.durBase } }
            }

            Image {
                anchors.fill: parent
                source: imageUrl
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: true
            }
        }

        Label {
            width: parent.width
            text: name
            color: Theme.text
            font.pixelSize: Theme.fontSm
            elide: Text.ElideRight
            maximumLineCount: 2
            wrapMode: Text.WordWrap
        }

        Label {
            width: parent.width
            text: role
            color: Theme.textMuted
            font.pixelSize: 10
            elide: Text.ElideRight
            visible: role.length > 0
        }
    }

    HoverHandler {
        id: hover
        cursorShape: card.linkable ? Qt.PointingHandCursor : Qt.ArrowCursor
    }
    TapHandler {
        enabled: card.linkable
        onTapped: card.openLink()
    }
}
