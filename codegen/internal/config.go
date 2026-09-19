package codegen

// This file is the single place to change the generation pattern.
//
// Everything here decides how Go declarations are NAMED and which ones are
// INCLUDED in the generated output. The generators themselves
// (generate_structs.go, generate_types.go, generate_ts_endpoints.go,
// generate_plugin_events.go) walk and emit; they do not make these choices.
//
// Changing a value here changes generated output, so run
//
//	go generate ./codegen
//
// from the repo root afterwards and commit the result. CI's "Codegen up to date"
// job fails if generated files and source disagree.

// space is the indent unit used by writeLine, which expands tabs in emitted
// TypeScript to this string.
const space = "    "

//////////////////////////////////////////////////////////////////////////////////////////////////////
// Type name prefixes
//////////////////////////////////////////////////////////////////////////////////////////////////////

// typePrefixesByPackage maps a Go package name to the prefix its types get in
// TypeScript, so that two packages declaring the same type name do not collide.
// e.g. package "models" + type "User" => "Models_User".
//
// A package mapped to "" is emitted unprefixed. A package absent from this map
// is also emitted unprefixed (see getTypePrefix), so adding an entry is only
// necessary to disambiguate or to match an existing frontend convention.
var typePrefixesByPackage = map[string]string{
	"anilist":                "AL_",
	"auto_downloader":        "AutoDownloader_",
	"autodownloader":         "AutoDownloader_",
	"entities":               "",
	"db":                     "DB_",
	"db_bridge":              "DB_",
	"models":                 "Models_",
	"playbackmanager":        "PlaybackManager_",
	"torrent_client":         "TorrentClient_",
	"events":                 "Events_",
	"torrent":                "Torrent_",
	"manga":                  "Manga_",
	"autoscanner":            "AutoScanner_",
	"listsync":               "ListSync_",
	"util":                   "Util_",
	"scanner":                "Scanner_",
	"offline":                "Offline_",
	"discordrpc":             "DiscordRPC_",
	"discordrpc_presence":    "DiscordRPC_",
	"anizip":                 "Anizip_",
	"animap":                 "Animap_",
	"onlinestream":           "Onlinestream_",
	"onlinestream_providers": "Onlinestream_",
	"onlinestream_sources":   "Onlinestream_",
	"manga_providers":        "Manga_",
	"chapter_downloader":     "ChapterDownloader_",
	"manga_downloader":       "MangaDownloader_",
	"docs":                   "INTERNAL_",
	"tvdb":                   "TVDB_",
	"metadata":               "Metadata_",
	"mappings":               "Mappings_",
	"mal":                    "MAL_",
	"handlers":               "",
	"updater":                "Updater_",
	"anime":                  "Anime_",
	"anime_types":            "Anime_",
	"summary":                "Summary_",
	"filesystem":             "Filesystem_",
	"filecache":              "Filecache_",
	"core":                   "INTERNAL_",
	"comparison":             "Comparison_",
	"mediastream":            "Mediastream_",
	"torrentstream":          "Torrentstream_",
	"extension":              "Extension_",
	"extension_repo":         "ExtensionRepo_",
	//"vendor_hibike_manga":        "HibikeManga_",
	//"vendor_hibike_onlinestream": "HibikeOnlinestream_",
	//"vendor_hibike_torrent":      "HibikeTorrent_",
	//"vendor_hibike_mediaplayer":  "HibikeMediaPlayer_",
	//"vendor_hibike_extension":    "HibikeExtension_",
	"hibikemanga":        "HibikeManga_",
	"hibikeonlinestream": "HibikeOnlinestream_",
	"hibiketorrent":      "HibikeTorrent_",
	"hibikemediaplayer":  "HibikeMediaPlayer_",
	"hibikeextension":    "HibikeExtension_",
	"hibikecustomsource": "HibikeCustomSource_",
	"continuity":         "Continuity_",
	"local":              "Local_",
	"debrid":             "Debrid_",
	"debrid_client":      "DebridClient_",
	"report":             "Report_",
	"habari":             "Habari_",
	"vendor_habari":      "Habari_",
	"discordrpc_client":  "DiscordRPC_",
	"directstream":       "Directstream_",
	"nativeplayer":       "NativePlayer_",
	"mpvcore":            "MpvCore_",
	"player":             "Player_",
	"mkvparser":          "MKVParser_",
	"nakama":             "Nakama_",
	"library_explorer":   "LibraryExplorer_",
	"customsource":       "CustomSource_",
	"videocore":          "VideoCore_",
	"plugin_ui":          "PluginUI_",
}

// getTypePrefix returns the TypeScript type prefix for a Go package name, or ""
// when the package is not listed in typePrefixesByPackage.
func getTypePrefix(packageName string) string {
	if prefix, ok := typePrefixesByPackage[packageName]; ok {
		return prefix
	}
	return ""
}

//////////////////////////////////////////////////////////////////////////////////////////////////////
// Extra structs to include
//////////////////////////////////////////////////////////////////////////////////////////////////////

// Structs that are not directly referenced by the API routes but are needed for the Typescript file.
var additionalStructNames = []string{
	"autoselect.StreamAutoSelectStatusPayload",
	"autoselect.AutoSelectCandidate",
	"torrentstream.TorrentLoadingStatus",
	"torrentstream.TorrentStatus",
	"debrid_client.StreamState",
	"extension_repo.TrayPluginExtensionItem",
	"vendor_habari.Metadata",
	"nativeplayer.PlaybackInfo",
	"nativeplayer.ServerEvent",
	"nativeplayer.ClientEvent",
	"nativeplayer.SubtitleEventsPayload",
	"mkvparser.SubtitleEvent",
	"nakama.NakamaStatus",
	"core.FeatureKey",
	"videocore.ClientEventType",
	"videocore.ServerEvent",
	"videocore.PlaybackState",
	"mpvcore.ClientEventType",
	"mpvcore.ServerEvent",
	"player.PlaybackInfo",
	"player.PlaybackState",
	"player.PlaybackStatus",
	"player.PlaylistState",
	"player.SubtitleTrack",
	"player.SkipData",
	"mpvcore.InSightData",
	"player.OnlinestreamParams",
	"plugin_ui.WebviewSlot",
	"plugin_ui.WebviewOptions",
	"videocore.InSightData",
}

// additionalStructNamesForEndpoints are extra structs imported by
// endpoint.types.ts beyond those reachable from handler params and body fields.
var additionalStructNamesForEndpoints = []string{}

var (
	additionalStructNamesForHooks = []string{
		"discordrpc_presence.MangaActivity",
		"discordrpc_presence.AnimeActivity",
		"discordrpc_presence.LegacyAnimeActivity",
		"discordrpc_presence.CustomActivity",
		"anilist.ListAnime",
		"anilist.ListManga",
		"anilist.MediaSort",
		"anilist.ListRecentAnime",
		"anilist.AnimeCollectionWithRelations",
		"onlinestream.Episode",
		"continuity.WatchHistoryItem",
		"continuity.WatchHistoryItemResponse",
		"continuity.UpdateWatchHistoryItemOptions",
		"continuity.WatchHistory",
		"torrent_client.Torrent",
	}
)

//////////////////////////////////////////////////////////////////////////////////////////////////////
// Endpoint keys
//////////////////////////////////////////////////////////////////////////////////////////////////////

// endpointKeyAcronyms repairs acronyms that getEndpointKey's camelCase-to-kebab
// split takes apart one letter at a time. "HandleGetTVDBEpisodes" becomes
// "get-t-v-d-b-episodes", and this table puts it back to "get-tvdb-episodes".
//
// Each replacement is applied once, in order. Add an entry when a new acronym
// appears in a handler name.
var endpointKeyAcronyms = []struct{ From, To string }{
	{"t-v-d-b", "tvdb"},
	{"m-a-l", "mal"},
}

//////////////////////////////////////////////////////////////////////////////////////////////////////
// Go to TypeScript scalar mapping
//////////////////////////////////////////////////////////////////////////////////////////////////////

// scalarGoToTS maps the Go scalar types that every conversion path agrees on to
// their TypeScript equivalent. It reports false for anything not a shared scalar.
//
// This is deliberately only the COMMON CORE. The three conversion functions that
// consult it each handle a few extra cases of their own and, more importantly,
// have different fallbacks for an unrecognized type:
//
//   - fieldTypeToTypescriptType  => getTypePrefix(pkg) + name  (also maps byte,
//     json.RawMessage, RawMessage)
//   - stringGoTypeToTypescriptType => the Go type unchanged  (also maps
//     json.RawMessage, RawMessage; deliberately NOT byte)
//   - goTypeToTypescriptType    => "unknown", which isCustomStruct treats as
//     "this is a struct, not a scalar"
//
// Those fallbacks are load-bearing, so do not widen this table to the union of
// all three without checking the generated diff: mapping "byte" here would make
// isCustomStruct("byte") false and change which fields become pointers.
func scalarGoToTS(goType string) (string, bool) {
	ts, ok := scalarGoToTSTable[goType]
	return ts, ok
}

var scalarGoToTSTable = map[string]string{
	"string":    "string",
	"bool":      "boolean",
	"nil":       "null",
	"time.Time": "string",

	"uint":    "number",
	"uint8":   "number",
	"uint16":  "number",
	"uint32":  "number",
	"uint64":  "number",
	"int":     "number",
	"int8":    "number",
	"int16":   "number",
	"int32":   "number",
	"int64":   "number",
	"float":   "number",
	"float32": "number",
	"float64": "number",
}

// tsRecordStringAny is the TypeScript type used for free-form JSON payloads.
const tsRecordStringAny = "Record<string, any>"

//////////////////////////////////////////////////////////////////////////////////////////////////////
// Zod schema generation
//////////////////////////////////////////////////////////////////////////////////////////////////////

// zodSchemaSuffix is appended to a generated type name to name its schema,
// e.g. type Models_User => const Models_UserSchema.
const zodSchemaSuffix = "Schema"

// zodUnknown is the fallback for a TypeScript expression the converter does not
// recognize. z.unknown() accepts anything, so an unmapped field is simply not
// validated rather than rejected.
const zodUnknown = "z.unknown()"

// zodNullishSuffix is applied to every optional field.
//
// It is .nullish() rather than .optional() because Go sends null, not "absent":
// codegen marks a field optional when it is a pointer, slice, map or qualified
// type, and a nil pointer/slice/map marshals to `"field": null`. More than half
// of all generated fields are optional this way, so .optional() alone would
// reject nearly every real response.
const zodNullishSuffix = ".nullish()"

// zodScalars maps the TypeScript primitives the generator emits to zod schemas.
//
// "any" becomes z.unknown() rather than z.any(): both accept anything, but
// z.unknown() keeps the inferred type honest about not knowing the shape.
var zodScalars = map[string]string{
	"string":              "z.string()",
	"number":              "z.number()",
	"boolean":             "z.boolean()",
	"any":                 "z.unknown()",
	"unknown":             "z.unknown()",
	"null":                "z.null()",
	"Record<string, any>": "z.record(z.string(), z.unknown())",
}
