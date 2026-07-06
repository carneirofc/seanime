import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Discover feed: several AniList carousels (trending, seasonal, upcoming, movies,
// missed sequels). Each row is a MediaCarousel that hides itself while empty.
Item {
    id: root

    Component.onCompleted: app.loadDiscover()

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            Label {
                text: "Discover"
                color: "#ffffff"
                font.pixelSize: 18
                font.bold: true
            }
            Item { Layout.fillWidth: true }
            Button {
                objectName: "discoverRefreshButton"
                text: "Refresh"
                onClicked: app.loadDiscover()
            }
        }

        Label {
            Layout.fillWidth: true
            visible: content.count === 0
            text: app.connectionStatus === "connected"
                  ? "Loading… (requires a working AniList login)"
                  : "Not connected."
            color: "#8a8a96"
            font.pixelSize: 16
            horizontalAlignment: Text.AlignHCenter
        }

        ScrollView {
            id: scroll
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentWidth: availableWidth
            clip: true

            ColumnLayout {
                id: content
                width: scroll.availableWidth
                spacing: 18

                // Sum of visible carousel rows, for the empty-state label.
                property int count: trending.count + season.count + prevSeason.count
                                    + upcoming.count + movies.count + missed.count

                MediaCarousel { id: trending; objectName: "discoverGrid"; title: "Trending Right Now"; model: app.discoverModel }
                MediaCarousel { id: season; title: "Top of the Season"; model: app.seasonModel }
                MediaCarousel { id: prevSeason; title: "Best of Last Season"; model: app.prevSeasonModel }
                MediaCarousel { id: upcoming; title: "Coming Soon"; model: app.upcomingModel }
                MediaCarousel { id: movies; title: "Trending Movies"; model: app.moviesModel }
                MediaCarousel { id: missed; title: "Missed Sequels"; model: app.missedSequelsModel }

                Item { Layout.preferredHeight: 8 }
            }
        }
    }
}
