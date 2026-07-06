import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Settings screen: a left section list and a stacked form pane per section.
//
// The first section, "Client", holds app-local prefs (server connection + AniList
// credentials) persisted on this machine via app.saveClientPrefs. The remaining
// sections mirror the Seanime server's settings object (app.settings) and are saved
// together in one PATCH via app.saveServerSettings.
//
// Save is safe against the server's wholesale-bind behaviour: each group is rebuilt
// from the current app.settings[group] with the edited fields overlaid, so fields the
// UI doesn't render are preserved rather than reset to zero.
Item {
    id: root

    property int currentIndex: 0

    // Registry of rendered server-setting fields, keyed "group.key" → SettingField.
    property var _fields: ({})
    function register(key, f) { _fields[key] = f }

    // Current value of a server setting, or a default while disconnected.
    function sv(group, key, def) {
        var g = (app.settings && app.settings[group]) ? app.settings[group] : ({})
        var v = g[key]
        return (v === undefined || v === null) ? def : v
    }

    // Rebuild a group from the live settings with the rendered edits overlaid.
    function grp(group) {
        var base = (app.settings && app.settings[group]) ? app.settings[group] : ({})
        var out = {}
        for (var k in base) out[k] = base[k]
        for (var fk in _fields) {
            var dot = fk.indexOf(".")
            if (fk.substring(0, dot) === group)
                out[fk.substring(dot + 1)] = _fields[fk].value
        }
        return out
    }

    function saveServer() {
        var groups = ["library", "mediaPlayer", "torrent", "anilist",
                      "discord", "manga", "notifications", "nakama"]
        var body = {}
        for (var i = 0; i < groups.length; i++) body[groups[i]] = grp(groups[i])
        app.saveServerSettings(body)
    }

    function saveClient() {
        app.saveClientPrefs(clientHost.value, clientPort.value, clientToken.value,
                            clientId.value, clientSecret.value)
    }

    readonly property bool connected: app.connectionStatus === "connected"

    // Server settings schema: one entry per group, driving the panes below.
    readonly property var serverSections: [
        {
            name: "Library", group: "library", fields: [
                { key: "libraryPath", label: "Library path", type: "text",
                  placeholder: "/path/to/anime" },
                { key: "autoScan", label: "Auto scan", type: "switch",
                  desc: "Rescan the library automatically when files change." },
                { key: "autoUpdateProgress", label: "Auto-update progress", type: "switch",
                  desc: "Mark episodes watched on AniList as you play them." },
                { key: "refreshLibraryOnStart", label: "Refresh library on start", type: "switch" },
                { key: "autoPlayNextEpisode", label: "Auto-play next episode", type: "switch" },
                { key: "enableWatchContinuity", label: "Watch continuity", type: "switch",
                  desc: "Resume playback from where you left off." },
                { key: "disableUpdateCheck", label: "Disable update check", type: "switch" },
                { key: "disableAnimeCardTrailers", label: "Disable card trailers", type: "switch" },
                { key: "enableManga", label: "Enable manga", type: "switch" },
                { key: "enableOnlinestream", label: "Enable online streaming", type: "switch" },
                { key: "useFallbackMetadataProvider", label: "Fallback metadata provider", type: "switch" },
                { key: "enableExtensionSecureMode", label: "Extension secure mode", type: "switch" },
                { key: "openTorrentClientOnStart", label: "Open torrent client on start", type: "switch" },
                { key: "openWebURLOnStart", label: "Open web UI on start", type: "switch" },
                { key: "torrentProvider", label: "Torrent provider", type: "text",
                  desc: "e.g. animetosho, nyaa, seadex, none." },
                { key: "defaultPlaybackSource", label: "Default playback source", type: "text",
                  desc: "library, torrentstream, debridstream, onlinestream." },
                { key: "dohProvider", label: "DoH provider", type: "text",
                  desc: "DNS-over-HTTPS provider (optional)." },
                { key: "updateChannel", label: "Update channel", type: "text",
                  desc: "github, seanime, or seanime_nightly." }
            ]
        },
        {
            name: "Media Player", group: "mediaPlayer", fields: [
                { key: "defaultPlayer", label: "Default player", type: "text",
                  desc: "mpv, vlc, mpc-hc, or iina." },
                { key: "host", label: "Player host", type: "text", placeholder: "127.0.0.1" },
                { key: "mpvPath", label: "mpv path", type: "text" },
                { key: "mpvSocket", label: "mpv socket", type: "text" },
                { key: "mpvArgs", label: "mpv args", type: "text" },
                { key: "vlcPath", label: "VLC path", type: "text" },
                { key: "vlcPort", label: "VLC port", type: "int", to: 65535 },
                { key: "vlcUsername", label: "VLC username", type: "text" },
                { key: "vlcPassword", label: "VLC password", type: "password" },
                { key: "mpcPath", label: "MPC-HC path", type: "text" },
                { key: "mpcPort", label: "MPC-HC port", type: "int", to: 65535 },
                { key: "iinaPath", label: "IINA path", type: "text" },
                { key: "iinaSocket", label: "IINA socket", type: "text" },
                { key: "iinaArgs", label: "IINA args", type: "text" },
                { key: "screenshotDir", label: "Screenshot directory", type: "text" },
                { key: "mpvPrismEnabled", label: "mpv Prism enabled", type: "switch" },
                { key: "mpvPrismLogging", label: "mpv Prism logging", type: "switch" },
                { key: "vcTranslate", label: "Voice/subtitle translate", type: "switch" },
                { key: "vcTranslateProvider", label: "Translate provider", type: "text" },
                { key: "vcTranslateTargetLanguage", label: "Translate target language", type: "text" },
                { key: "vcTranslateModel", label: "Translate model", type: "text" },
                { key: "vcTranslateBaseUrl", label: "Translate base URL", type: "text" },
                { key: "vcTranslateApiKey", label: "Translate API key", type: "password" }
            ]
        },
        {
            name: "Torrent", group: "torrent", fields: [
                { key: "defaultTorrentClient", label: "Default client", type: "text",
                  desc: "qbittorrent, transmission, or none." },
                { key: "qbittorrentHost", label: "qBittorrent host", type: "text" },
                { key: "qbittorrentPort", label: "qBittorrent port", type: "int", to: 65535 },
                { key: "qbittorrentUsername", label: "qBittorrent username", type: "text" },
                { key: "qbittorrentPassword", label: "qBittorrent password", type: "password" },
                { key: "qbittorrentPath", label: "qBittorrent path", type: "text" },
                { key: "qbittorrentTags", label: "qBittorrent tags", type: "text" },
                { key: "qbittorrentCategory", label: "qBittorrent category", type: "text" },
                { key: "transmissionHost", label: "Transmission host", type: "text" },
                { key: "transmissionPort", label: "Transmission port", type: "int", to: 65535 },
                { key: "transmissionUsername", label: "Transmission username", type: "text" },
                { key: "transmissionPassword", label: "Transmission password", type: "password" },
                { key: "transmissionPath", label: "Transmission path", type: "text" },
                { key: "seanimePort", label: "Seanime client port", type: "int", to: 65535 },
                { key: "seanimeMaxConnections", label: "Max connections", type: "int" },
                { key: "seanimeDownloadLimit", label: "Download limit (KB/s)", type: "int" },
                { key: "seanimeUploadLimit", label: "Upload limit (KB/s)", type: "int" },
                { key: "seanimeMaxActiveDownloads", label: "Max active downloads", type: "int" },
                { key: "showActiveTorrentCount", label: "Show active torrent count", type: "switch" }
            ]
        },
        {
            name: "AniList", group: "anilist", fields: [
                { key: "hideAudienceScore", label: "Hide audience score", type: "switch" },
                { key: "enableAdultContent", label: "Enable adult content", type: "switch" },
                { key: "blurAdultContent", label: "Blur adult content", type: "switch" },
                { key: "splitAdultContent", label: "Split adult content", type: "switch" },
                { key: "hideMediaTagsSpoilers", label: "Hide tag spoilers", type: "switch" },
                { key: "disableCacheLayer", label: "Disable cache layer", type: "switch" }
            ]
        },
        {
            name: "Discord", group: "discord", fields: [
                { key: "enableRichPresence", label: "Rich presence", type: "switch" },
                { key: "enableAnimeRichPresence", label: "Anime rich presence", type: "switch" },
                { key: "enableMangaRichPresence", label: "Manga rich presence", type: "switch" },
                { key: "richPresenceUseMediaTitleStatus", label: "Use media title as status", type: "switch" },
                { key: "richPresenceShowAniListMediaButton", label: "Show AniList media button", type: "switch" },
                { key: "richPresenceShowAniListProfileButton", label: "Show AniList profile button", type: "switch" },
                { key: "richPresenceHideSeanimeRepositoryButton", label: "Hide Seanime repo button", type: "switch" }
            ]
        },
        {
            name: "Manga", group: "manga", fields: [
                { key: "defaultMangaProvider", label: "Default provider", type: "text" },
                { key: "mangaLocalSourceDirectory", label: "Local source directory", type: "text" },
                { key: "mangaAutoUpdateProgress", label: "Auto-update progress", type: "switch" },
                { key: "mangaCacheDurationHours", label: "Cache duration (hours)", type: "int",
                  desc: "0 = never expire." }
            ]
        },
        {
            name: "Notifications", group: "notifications", fields: [
                { key: "disableNotifications", label: "Disable all notifications", type: "switch" },
                { key: "disableAutoDownloaderNotifications", label: "Disable auto-downloader notifications", type: "switch" },
                { key: "disableAutoScannerNotifications", label: "Disable auto-scanner notifications", type: "switch" }
            ]
        },
        {
            name: "Nakama", group: "nakama", fields: [
                { key: "enabled", label: "Enable Nakama", type: "switch" },
                { key: "username", label: "Username", type: "text" },
                { key: "isHost", label: "Act as host", type: "switch" },
                { key: "hostPassword", label: "Host password", type: "password" },
                { key: "remoteServerURL", label: "Remote server URL", type: "text" },
                { key: "remoteServerPassword", label: "Remote server password", type: "password" },
                { key: "includeNakamaAnimeLibrary", label: "Include host anime library", type: "switch" },
                { key: "hostShareLocalAnimeLibrary", label: "Share local anime library", type: "switch" },
                { key: "hostEnablePortForwarding", label: "Enable port forwarding", type: "switch" }
            ]
        }
    ]

    // Section names for the left nav: Client first, then the server groups.
    readonly property var navNames: {
        var names = ["Client"]
        for (var i = 0; i < serverSections.length; i++) names.push(serverSections[i].name)
        return names
    }

    RowLayout {
        anchors.fill: parent
        spacing: 0

        // ---- left section nav ----
        Rectangle {
            Layout.preferredWidth: 180
            Layout.fillHeight: true
            color: Theme.surfaceAlt

            ColumnLayout {
                anchors.fill: parent
                anchors.topMargin: Theme.spacing
                spacing: 2

                Label {
                    text: "Settings"
                    color: Theme.textStrong
                    font.pixelSize: Theme.fontXl
                    font.bold: true
                    Layout.leftMargin: Theme.spacingLg
                    Layout.bottomMargin: Theme.spacingSm
                }

                Repeater {
                    model: root.navNames
                    delegate: Rectangle {
                        id: navItem
                        required property int index
                        required property string modelData
                        objectName: "settingsNav_" + modelData
                        Layout.fillWidth: true
                        Layout.preferredHeight: 38
                        Layout.leftMargin: Theme.spacingSm
                        Layout.rightMargin: Theme.spacingSm
                        radius: Theme.radius
                        readonly property bool active: root.currentIndex === index
                        color: active ? Theme.elevated
                             : (hover.hovered ? Theme.surfaceHover : "transparent")

                        Label {
                            anchors.left: parent.left
                            anchors.leftMargin: Theme.spacing
                            anchors.verticalCenter: parent.verticalCenter
                            text: navItem.modelData
                            color: navItem.active ? Theme.textStrong : Theme.textDim
                            font.pixelSize: Theme.fontBase
                            font.bold: navItem.active
                        }

                        HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: root.currentIndex = navItem.index }
                    }
                }

                Item { Layout.fillHeight: true }
            }
        }

        // ---- right form area ----
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 0

            StackLayout {
                id: panes
                Layout.fillWidth: true
                Layout.fillHeight: true
                currentIndex: root.currentIndex

                // Index 0 — Client (app-local prefs).
                Flickable {
                    contentHeight: clientCol.implicitHeight + 2 * Theme.spacingLg
                    clip: true
                    ScrollBar.vertical: ScrollBar {}
                    ColumnLayout {
                        id: clientCol
                        x: Theme.spacingLg
                        y: Theme.spacingLg
                        width: panes.width - 2 * Theme.spacingLg
                        spacing: Theme.spacing

                        Label {
                            text: "These settings are stored on this computer."
                            color: Theme.textMuted
                            font.pixelSize: Theme.fontSm
                            Layout.fillWidth: true
                            wrapMode: Text.WordWrap
                        }

                        SettingField {
                            id: clientHost; label: "Server host"; type: "text"
                            placeholder: "127.0.0.1"; value: app.serverHost
                        }
                        SettingField {
                            id: clientPort; label: "Server port"; type: "text"
                            placeholder: "43211"; value: app.serverPort
                        }
                        SettingField {
                            id: clientToken; label: "Server token"; type: "password"
                            description: "Only needed for password-protected servers."
                            value: app.serverToken
                        }
                        SettingField {
                            id: clientId; label: "AniList client ID"; type: "text"
                            value: app.anilistClientId
                        }
                        SettingField {
                            id: clientSecret; label: "AniList client secret"; type: "password"
                            description: "Required for the AniList login exchange."
                            value: app.anilistClientSecret
                        }
                    }
                }

                // Indices 1..N — server setting groups.
                Repeater {
                    model: root.serverSections
                    delegate: Flickable {
                        id: pane
                        required property var modelData
                        contentHeight: col.implicitHeight + 2 * Theme.spacingLg
                        clip: true
                        ScrollBar.vertical: ScrollBar {}

                        ColumnLayout {
                            id: col
                            x: Theme.spacingLg
                            y: Theme.spacingLg
                            width: pane.width - 2 * Theme.spacingLg
                            spacing: Theme.spacing

                            Label {
                                visible: !root.connected
                                text: "Connect to a server to load and edit these settings."
                                color: Theme.warnText
                                font.pixelSize: Theme.fontSm
                                Layout.fillWidth: true
                                wrapMode: Text.WordWrap
                            }

                            Repeater {
                                model: pane.modelData.fields
                                delegate: SettingField {
                                    required property var modelData
                                    label: modelData.label
                                    description: modelData.desc || ""
                                    type: modelData.type
                                    from: modelData.from || 0
                                    to: modelData.to || 999999
                                    placeholder: modelData.placeholder || ""
                                    value: root.sv(pane.modelData.group, modelData.key,
                                                   modelData.type === "switch" ? false
                                                 : modelData.type === "int" ? 0 : "")
                                    Component.onCompleted:
                                        root.register(pane.modelData.group + "." + modelData.key, this)
                                }
                            }
                        }
                    }
                }
            }

            // ---- save bar ----
            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 52
                color: Theme.surfaceAlt

                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: Theme.spacingLg
                    anchors.rightMargin: Theme.spacingLg
                    spacing: Theme.spacing

                    Label {
                        id: savedToast
                        objectName: "settingsSavedToast"
                        text: "Settings saved"
                        color: Theme.successText
                        font.pixelSize: Theme.fontMd
                        opacity: 0
                        Behavior on opacity { NumberAnimation { duration: Theme.durBase } }
                    }

                    Item { Layout.fillWidth: true }

                    AppButton {
                        objectName: "saveSettingsButton"
                        text: root.currentIndex === 0 ? "Save & reconnect" : "Save settings"
                        enabled: root.currentIndex === 0 || root.connected
                        onClicked: root.currentIndex === 0 ? root.saveClient() : root.saveServer()
                    }
                }
            }
        }
    }

    // Flash the "Settings saved" toast when a server PATCH succeeds.
    Timer { id: toastTimer; interval: 2000; onTriggered: savedToast.opacity = 0 }
    Connections {
        target: app
        function onSettingsSaved() { savedToast.opacity = 1; toastTimer.restart() }
    }
}
