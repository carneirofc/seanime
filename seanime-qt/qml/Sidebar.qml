import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Left navigation, mirroring the web frontend: logo on top, a vertical
// icon+label menu, and a user/login chip pinned to the bottom.
//
// Collapsible: the header toggle shrinks the sidebar to a narrow icon-only
// rail (labels hide, icons/avatar/status-dot centre, hovering an item shows
// its label as a tooltip) and expands it back. The width animates, and the
// layout in Main.qml tracks `implicitWidth`, so the content area reflows with it.
Rectangle {
    id: sidebar
    color: Theme.surfaceAlt
    // Clip so labels don't spill outside the rail while the width animates.
    clip: true

    // Current top-level page id, driven by Main; used to highlight the active item.
    property string currentPage: "home"

    // Collapsed = icon-only rail. Session-only (not persisted).
    property bool collapsed: false
    readonly property int expandedWidth: 210
    readonly property int collapsedWidth: 64

    // Drive the layout width off this (Main binds Layout.preferredWidth to it) so
    // the whole content area animates along with the sidebar collapse/expand.
    implicitWidth: collapsed ? collapsedWidth : expandedWidth
    Behavior on implicitWidth { NumberAnimation { duration: Theme.durBase; easing.type: Theme.easeStandard } }

    signal navigate(string page)
    signal loginRequested()
    // Emitted by the connection-status link to jump to the server-connection settings.
    signal serverSettingsRequested()

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // ---- logo + collapse toggle ----
        Item {
            Layout.fillWidth: true
            Layout.preferredHeight: 64

            Label {
                anchors.left: parent.left
                anchors.leftMargin: Theme.spacing
                anchors.verticalCenter: parent.verticalCenter
                text: "Seanime"
                color: Theme.textStrong
                font.pixelSize: Theme.fontXxl
                font.bold: true
                // Fade out (and drop from hit-testing) as the rail collapses.
                visible: !sidebar.collapsed
                opacity: sidebar.collapsed ? 0 : 1
                Behavior on opacity { NumberAnimation { duration: Theme.durFast } }
            }

            AppToolButton {
                id: collapseBtn
                objectName: "collapseToggle"
                anchors.verticalCenter: parent.verticalCenter
                // Pinned right when expanded; centred when it's the only thing showing.
                anchors.right: sidebar.collapsed ? undefined : parent.right
                anchors.rightMargin: Theme.spacingSm
                anchors.horizontalCenter: sidebar.collapsed ? parent.horizontalCenter : undefined
                iconName: sidebar.collapsed ? "chevron-right" : "chevron-left"
                onClicked: sidebar.collapsed = !sidebar.collapsed

                Accessible.name: sidebar.collapsed ? "Expand sidebar" : "Collapse sidebar"
                Accessible.description: "Toggle the navigation sidebar between full and icon-only"
                ToolTip.visible: hovered
                ToolTip.delay: 400
                ToolTip.text: sidebar.collapsed ? "Expand sidebar" : "Collapse sidebar"
            }
        }

        // ---- nav items ----
        Repeater {
            model: [
                { pageId: "home",     label: "Home",     icon: "home" },
                { pageId: "manga",    label: "Manga",    icon: "books" },
                { pageId: "discover", label: "Discover", icon: "compass" },
                { pageId: "search",   label: "Search",   icon: "search" },
                { pageId: "extensions", label: "Extensions", icon: "puzzle" },
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

                // In the collapsed rail the label is hidden, so surface it as a tooltip.
                ToolTip.visible: sidebar.collapsed && navHover.hovered
                ToolTip.delay: 400
                ToolTip.text: modelData.label

                // Visible focus ring for keyboard users (fades in/out).
                border.width: navItem.activeFocus ? 2 : 0
                border.color: Theme.accent
                Behavior on border.width { NumberAnimation { duration: Theme.durFast } }

                Row {
                    // Left-aligned when expanded; centred (icon only) when collapsed.
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.left: sidebar.collapsed ? undefined : parent.left
                    anchors.leftMargin: Theme.spacing
                    anchors.horizontalCenter: sidebar.collapsed ? parent.horizontalCenter : undefined
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
                        visible: !sidebar.collapsed
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

        // ---- connection status (also a link into the server-connection settings) ----
        Rectangle {
            id: connStatus
            objectName: "connectionStatus"
            Layout.fillWidth: true
            Layout.leftMargin: Theme.spacingSm
            Layout.rightMargin: Theme.spacingSm
            Layout.bottomMargin: 4
            Layout.preferredHeight: 30
            radius: Theme.radius
            color: connHover.hovered ? Theme.surfaceHover : "transparent"
            Behavior on color { ColorAnimation { duration: Theme.durFast } }

            function activate() { sidebar.serverSettingsRequested() }

            // Keyboard reachable + activatable.
            activeFocusOnTab: true
            Keys.onReturnPressed: connStatus.activate()
            Keys.onEnterPressed: connStatus.activate()
            Keys.onSpacePressed: connStatus.activate()

            // Accessibility: a button describing the state and its action.
            Accessible.role: Accessible.Button
            Accessible.name: "Server: " + app.connectionStatus
            Accessible.description: "Open the server connection settings"
            Accessible.focusable: true
            Accessible.onPressAction: connStatus.activate()

            // Collapsed rail hides the status text, so show it on hover.
            ToolTip.visible: sidebar.collapsed && connHover.hovered
            ToolTip.delay: 400
            ToolTip.text: "Server: " + app.connectionStatus

            border.width: connStatus.activeFocus ? 2 : 0
            border.color: Theme.accent
            Behavior on border.width { NumberAnimation { duration: Theme.durFast } }

            Row {
                anchors.verticalCenter: parent.verticalCenter
                anchors.left: sidebar.collapsed ? undefined : parent.left
                anchors.leftMargin: 12
                anchors.horizontalCenter: sidebar.collapsed ? parent.horizontalCenter : undefined
                spacing: 8
                Rectangle {
                    width: 9; height: 9; radius: 5
                    anchors.verticalCenter: parent.verticalCenter
                    color: app.connectionStatus === "connected" ? Theme.success
                         : app.connectionStatus === "connecting" ? Theme.warning
                         : Theme.danger
                    Behavior on color { ColorAnimation { duration: Theme.durBase } }
                }
                Label {
                    text: app.connectionStatus
                    visible: !sidebar.collapsed
                    anchors.verticalCenter: parent.verticalCenter
                    // Accent on hover to read as a link; muted otherwise.
                    color: connHover.hovered ? Theme.accent : Theme.textMuted
                    font.pixelSize: Theme.fontSm
                    font.underline: connHover.hovered
                    Behavior on color { ColorAnimation { duration: Theme.durFast } }
                }
            }

            HoverHandler { id: connHover; cursorShape: Qt.PointingHandCursor }
            TapHandler { onTapped: connStatus.activate() }
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

            // Collapsed rail shows only the avatar, so surface the name/action on hover.
            ToolTip.visible: sidebar.collapsed && userHover.hovered
            ToolTip.delay: 400
            ToolTip.text: userChip.loggedIn ? app.username : "Log in with AniList"

            border.width: userChip.activeFocus ? 2 : 0
            border.color: Theme.accent
            Behavior on border.width { NumberAnimation { duration: Theme.durFast } }

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 10
                anchors.rightMargin: 10
                spacing: 10

                // Leading/trailing spacers exist only in the collapsed rail; they
                // centre the avatar. When expanded they're invisible (excluded from
                // the layout), so the original left-aligned layout is unchanged.
                Item { Layout.fillWidth: true; visible: sidebar.collapsed }

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
                    visible: !sidebar.collapsed
                    text: userChip.loggedIn ? app.username : "Log in with AniList"
                    color: userChip.loggedIn ? Theme.text : Theme.accentSoft
                    font.pixelSize: Theme.fontMd
                    elide: Text.ElideRight
                }

                Item { Layout.fillWidth: true; visible: sidebar.collapsed }
            }

            HoverHandler { id: userHover; cursorShape: Qt.PointingHandCursor }
            TapHandler { onTapped: userChip.activate() }
        }
    }
}
