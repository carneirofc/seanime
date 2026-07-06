import QtQuick
import QtQuick.Controls
import QtQuick.Effects

// A small character portrait + name + role, used in the DetailView strip.
Item {
    id: card

    // Gentle lift on hover for a bit of life in the strip.
    scale: hover.hovered ? 1.04 : 1.0
    z: hover.hovered ? 1 : 0
    Behavior on scale { NumberAnimation { duration: Theme.durBase; easing.type: Theme.easeEmphasis } }

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

    HoverHandler { id: hover }
}
