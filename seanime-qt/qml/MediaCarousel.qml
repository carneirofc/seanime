import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Seanime 1.0

// A titled preview feed, reused by DiscoverView (each feed) and DetailView
// (relations/recommendations). When the server's "split adult content" setting is
// off it shows a single strip (adult titles mixed in, blurred per-card); when on
// it shows a safe strip and, beneath it, a separate "· adult" strip, each fed by
// an AdultFilterProxy over the same source model so nothing mixes.
ColumnLayout {
    id: root
    property string title: ""
    property var model: null

    readonly property bool split: app.splitAdultContent

    // Total items across the feed regardless of split — only the active strips
    // hold a model, so summing their (reactive) counts gives the source total.
    // DiscoverView sums this across feeds for its empty-state label.
    readonly property int count: mixed.count + safe.count + adult.count

    spacing: 12
    visible: count > 0
    Layout.fillWidth: true

    // Paired halves of the source; only consulted while split is on.
    AdultFilterProxy { id: safeProxy; sourceModel: root.model; wantAdult: false }
    AdultFilterProxy { id: adultProxy; sourceModel: root.model; wantAdult: true }

    // Only the strips in play get a model, so the others create no delegates.
    CarouselStrip {
        id: mixed
        title: root.title
        model: root.split ? null : root.model
    }
    CarouselStrip {
        id: safe
        title: root.title
        model: root.split ? safeProxy : null
    }
    CarouselStrip {
        id: adult
        title: root.title + " · adult"
        model: root.split ? adultProxy : null
    }
}
