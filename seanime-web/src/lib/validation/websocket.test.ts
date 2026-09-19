import { describe, expect, it } from "vitest"
import { parsePluginBatchEvents, parsePluginEnvelope, parseWebsocketFrame } from "./websocket"

describe("websocket frame validation", () => {
    it("parses a well-formed frame", () => {
        const frame = parseWebsocketFrame(JSON.stringify({ type: "scan-progress", payload: { value: 40 } }), "test")

        expect(frame).toEqual({ type: "scan-progress", payload: { value: 40 } })
    })

    it("keeps a frame whose payload is absent", () => {
        // Plenty of server events are notifications with no body.
        expect(parseWebsocketFrame(JSON.stringify({ type: "refresh" }), "test")).toEqual({ type: "refresh" })
    })

    it.each([
        ["not JSON at all", "}{"],
        ["a frame with no type", JSON.stringify({ payload: 1 })],
        ["a frame with an empty type", JSON.stringify({ type: "", payload: 1 })],
        ["a frame with a non-string type", JSON.stringify({ type: 7, payload: 1 })],
        ["a JSON array", JSON.stringify([1, 2, 3])],
        ["a JSON scalar", JSON.stringify("hello")],
    ])("drops %s without throwing", (_label, raw) => {
        expect(parseWebsocketFrame(raw, "test")).toBeNull()
    })

    it("drops a frame whose data is not a string", () => {
        expect(parseWebsocketFrame(new ArrayBuffer(8), "test")).toBeNull()
    })
})

describe("plugin envelope validation", () => {
    it("defaults a missing extensionId to empty", () => {
        expect(parsePluginEnvelope({ type: "plugin:event", payload: { a: 1 } }, "test")).toEqual({
            type: "plugin:event",
            extensionId: "",
            payload: { a: 1 },
        })
    })

    it("drops an envelope with no type", () => {
        expect(parsePluginEnvelope({ extensionId: "x", payload: {} }, "test")).toBeNull()
    })
})

describe("plugin batch events", () => {
    it("returns the events of a well-formed batch", () => {
        const events = parsePluginBatchEvents(
            { events: [{ type: "a", extensionId: "ext", payload: 1 }, { type: "b", extensionId: "ext", payload: 2 }] },
            "test",
        )

        expect(events).toEqual([
            { type: "a", extensionId: "ext", payload: 1 },
            { type: "b", extensionId: "ext", payload: 2 },
        ])
    })

    it("drops only the malformed entries, keeping the rest", () => {
        // A third-party plugin sending one bad event must not discard the whole batch.
        const events = parsePluginBatchEvents(
            { events: [{ type: "good", extensionId: "ext", payload: 1 }, "not-an-event", { noType: true }] },
            "test",
        )

        expect(events).toEqual([{ type: "good", extensionId: "ext", payload: 1 }])
    })

    it.each([
        ["events is not an array", { events: "nope" }],
        ["events is missing", {}],
        ["the payload is not an object", "nope"],
        ["the payload is null", null],
    ])("returns an empty list when %s", (_label, payload) => {
        expect(parsePluginBatchEvents(payload, "test")).toEqual([])
    })
})
