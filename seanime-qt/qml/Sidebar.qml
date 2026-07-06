import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Left navigation, mirroring the web frontend: logo on top, a vertical
// icon+label menu, and a user/login chip pinned to the bottom.
Rectangle {
    id: sidebar
    color: Theme.surfaceAlt

    // Current top-level page id, driven by Main; used to highlight the active item.
    property string currentPage: "home"

    signal navigate(string page)
    signal loginRequested()

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // ---- logo ----
        Item {
            Layout.fillWidth: true
            Layout.preferredHeight: 64
            Label {
                anchors.centerIn: parent
                text: "Seanime"
                color: Theme.textStrong
                font.pixelSize: Theme.fontXxl
                font.bold: true
            }
        }

        // ---- nav items ----
        Repeater {
            model: [
                { pageId: "home",     label: "Home",     icon: "home" },
                { pageId: "manga",    label: "Manga",    icon: "books" },
                { pageId: "discover", label: "Discover", icon: "compass" },
                { pageId: "search",   label: "Search",   icon: "search" },
                { pageId: "profile",  label: "Profile",  icon: "user" },
                { pageId: "settings", label: "Settings", icon: "settings" },
            ]
            delegate: Rectangle {
                id: navItem
                required property var modelData
                objectName: "nav_" + modelData.pageId
                Layout.fillWidth: true
                Layout.preferredHeight: 44
                Layout.leftMargin: Theme.spacingSm
                Layout.rightMargin: Theme.spacingSm
                radius: Theme.radius
                readonly property bool active: sidebar.currentPage === modelData.pageId
                color: active ? Theme.elevated : (navHover.hovered ? Theme.surfaceHover : "transparent")

                // Smooth hover / active colour changes.
                Behavior on color { ColorAnimation { duration: Theme.durFast } }

                function activate() { sidebar.navigate(modelData.pageId) }

                // Keyboard: reachable via Tab, activated with Enter/Return/Space.
                activeFocusOnTab: true
                Keys.onReturnPressed: navItem.activate()
                Keys.onEnterPressed: navItem.activate()
                Keys.onSpacePressed: navItem.activate()

                // Accessibility: announce as a button with its label; expose the
                // press action so assistive tech can trigger navigation.
                Accessible.role: Accessible.Button
                Accessible.name: modelData.label
                Accessible.description: "Go to the " + modelData.label + " page"
                Accessible.focusable: true
                Accessible.onPressAction: navItem.activate()

                // Visible focus ring for keyboard users (fades in/out).
                border.width: navItem.activeFocus ? 2 : 0
                border.color: Theme.accent
                Behavior on border.width { NumberAnimation { duration: Theme.durFast } }

                Row {
                    anchors.left: parent.left
                    anchors.leftMargin: Theme.spacing
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Theme.spacing
                    Icon {
                        name: modelData.icon
                        size: 19
                        color: active ? Theme.textStrong : Theme.textDim
                        anchors.verticalCenter: parent.verticalCenter
                        Behavior on color { ColorAnimation { duration: Theme.durFast } }
                    }
                    Label {
                        text: modelData.label
                        color: active ? Theme.textStrong : Theme.textDim
                        font.pixelSize: Theme.fontBase
                        font.bold: active
                        anchors.verticalCenter: parent.verticalCenter
                        Behavior on color { ColorAnimation { duration: Theme.durFast } }
                    }
                }

                // Active accent bar on the left edge — grows in with a little
                // overshoot when the item becomes active.
                Rectangle {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: 3
                    height: navItem.active ? parent.height * 0.5 : 0
                    radius: 2
                    color: Theme.accent
                    opacity: navItem.active ? 1 : 0
                    Behavior on height { NumberAnimation { duration: Theme.durBase; easing.type: Theme.easeEmphasis } }
                    Behavior on opacity { NumberAnimation { duration: Theme.durBase } }
                }

                HoverHandler { id: navHover; cursorShape: Qt.PointingHandCursor }
                TapHandler { onTapped: navItem.activate() }
            }
        }

        Item { Layout.fillHeight: true }  // push the rest to the bottom

        // ---- connection status ----
        RowLayout {
            Layout.fillWidth: true
            Layout.leftMargin: 20
            Layout.rightMargin: 12
            Layout.bottomMargin: 8
            spacing: 8
            Rectangle {
                width: 9; height: 9; radius: 5
                color: app.connectionStatus === "connected" ? Theme.success
                     : app.connectionStatus === "connecting" ? Theme.warning
                     : Theme.danger
                Behavior on color { ColorAnimation { duration: Theme.durBase } }
            }
            Label {
                text: app.connectionStatus
                color: Theme.textMuted
                font.pixelSize: Theme.fontSm
            }
        }

        // ---- user / login chip ----
        Rectangle {
            id: userChip
            objectName: "userChip"
            Layout.fillWidth: true
            Layout.preferredHeight: 56
            Layout.margins: Theme.spacingSm
            radius: Theme.radius
            readonly property bool loggedIn: app.username.length > 0
            readonly property bool hasAvatar: loggedIn && app.avatarUrl.length > 0
            color: userHover.hovered ? Theme.surfaceHover : Theme.surface
            Behavior on color { ColorAnimation { duration: Theme.durFast } }

            function activate() {
                if (loggedIn) sidebar.navigate("profile")
                else sidebar.loginRequested()
            }

            // Keyboard reachable + activatable.
            activeFocusOnTab: true
            Keys.onReturnPressed: userChip.activate()
            Keys.onEnterPressed: userChip.activate()
            Keys.onSpacePressed: userChip.activate()

            // Accessibility: name reflects the current state/action.
            Accessible.role: Accessible.Button
            Accessible.name: loggedIn ? app.username : "Log in with AniList"
            Accessible.description: loggedIn
                ? "Open your profile"
                : "Sign in to AniList"
            Accessible.focusable: true
            Accessible.onPressAction: userChip.activate()

            border.width: userChip.activeFocus ? 2 : 0
            border.color: Theme.accent
            Behavior on border.width { NumberAnimation { duration: Theme.durFast } }

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 10
                anchors.rightMargin: 10
                spacing: 10

                // Avatar (or a login glyph when signed out).
                Rectangle {
                    width: 34; height: 34; radius: 17
                    color: Theme.inset
                    clip: true
                    Image {
                        anchors.fill: parent
                        source: app.avatarUrl
                        visible: userChip.hasAvatar
                        fillMode: Image.PreserveAspectCrop
                        asynchronous: true
                    }
                    Label {
                        anchors.centerIn: parent
                        visible: !userChip.hasAvatar && userChip.loggedIn
                        text: app.username.charAt(0).toUpperCase()
                        color: Theme.textMuted
                        font.pixelSize: 16
                    }
                    Icon {
                        anchors.centerIn: parent
                        visible: !userChip.hasAvatar && !userChip.loggedIn
                        name: "key"
                        size: 16
                        color: Theme.textMuted
                    }
                }

                Label {
                    Layout.fillWidth: true
                    text: userChip.loggedIn ? app.username : "Log in with AniList"
                    color: userChip.loggedIn ? Theme.text : Theme.accentSoft
                    font.pixelSize: Theme.fontMd
                    elide: Text.ElideRight
                }
            }

            HoverHandler { id: userHover; cursorShape: Qt.PointingHandCursor }
            TapHandler { onTapped: userChip.activate() }
        }
    }
}
