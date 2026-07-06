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

    // Adult / score metadata come from the model as context roles, but not every
    // model that reuses this delegate defines them — guard with typeof so a
    // missing role reads as a safe default rather than a ReferenceError.
    readonly property bool cardIsAdult: (typeof isAdult !== "undefined") && isAdult === true
    readonly property int cardScore: (typeof score !== "undefined" && score) ? score : 0

    // Poster is hidden behind a blur when it's adult content and the server asks
    // for it — until the user clicks to reveal this individual card.
    property bool revealed: false
    readonly property bool blurred: cardIsAdult && app.blurAdultContent && !revealed

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

        // Title is pinned to the bottom (reserving its own font-scaled height) and
        // the poster well fills the space above it, so posters grow and shrink
        // proportionally with the grid cell (Theme.posterScale) rather than sitting
        // at a fixed height that would leave dead space when the cell is enlarged.
        Item {
            anchors.fill: parent
            anchors.margins: 8

            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: titleLabel.top
                anchors.bottomMargin: 6
                radius: 6
                clip: true
                color: Theme.inset

                Image {
                    id: poster
                    anchors.fill: parent
                    source: posterUrl
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                }

                // Adult blur: a blurred copy of the poster drawn on top until the
                // user reveals it. Sourcing the (still-rendered) Image keeps this
                // in sync with lazy poster loading.
                MultiEffect {
                    anchors.fill: parent
                    source: poster
                    visible: card.blurred
                    blurEnabled: true
                    blur: 1.0
                    blurMax: 48
                    autoPaddingEnabled: false
                }

                // Reveal affordance over a blurred poster.
                Rectangle {
                    anchors.fill: parent
                    visible: card.blurred
                    color: "#40000000"
                    Column {
                        anchors.centerIn: parent
                        spacing: 4
                        Icon {
                            anchors.horizontalCenter: parent.horizontalCenter
                            name: "rating-18-plus"
                            size: 28
                            color: Theme.textStrong
                        }
                        Label {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: "Click to reveal"
                            color: Theme.textStrong
                            font.pixelSize: Theme.fontXs
                        }
                    }
                }

                // Score badge (top-left) when AniList has a mean score.
                Rectangle {
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.margins: 4
                    visible: card.cardScore > 0 && !card.blurred
                    height: 20
                    width: scoreBadge.implicitWidth + 12
                    radius: Theme.radiusSm
                    color: Theme.overlay
                    Row {
                        id: scoreBadge
                        anchors.centerIn: parent
                        spacing: 3
                        Icon {
                            name: "star"
                            size: Theme.fontXs
                            color: Theme.warnText
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Label {
                            text: card.cardScore + "%"
                            color: Theme.warnText
                            font.pixelSize: Theme.fontXs
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                }

                // Progress / episode-count badge.
                Rectangle {
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: 4
                    visible: !card.blurred
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

                // Adult marker (bottom-right) so revealed adult posters stay flagged.
                Rectangle {
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.margins: 4
                    visible: card.cardIsAdult && !card.blurred
                    height: 18
                    width: adultBadge.implicitWidth + 10
                    radius: Theme.radiusSm
                    color: Theme.dangerFill
                    Label {
                        id: adultBadge
                        anchors.centerIn: parent
                        text: "18+"
                        color: Theme.dangerText
                        font.pixelSize: Theme.fontXs
                        font.bold: true
                    }
                }
            }

            Label {
                id: titleLabel
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
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
    // A blurred adult poster reveals on first tap; a normal (or revealed) card opens.
    TapHandler { onTapped: card.blurred ? (card.revealed = true) : card.activate() }
}
