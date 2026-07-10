package mpvcore

import (
	"seanime/internal/api/metadata_provider"
	"seanime/internal/continuity"
	discordrpc_presence "seanime/internal/discordrpc/presence"
	"seanime/internal/events"
	"seanime/internal/platforms/platform"
	"seanime/internal/util"

	"github.com/rs/zerolog"
)

type (
	MpvCore struct {
		wsEventManager events.WSEventManagerInterface

		continuityManager          *continuity.Manager
		metadataProviderRef        *util.Ref[metadata_provider.Provider]
		discordPresence            *discordrpc_presence.Presence
		platformRef                *util.Ref[platform.Platform]
		refreshAnimeCollectionFunc func() // This function is called to refresh the AniList collection
		isOfflineRef               *util.Ref[bool]
	}

	NewMpvCoreOptions struct {
		WsEventManager             events.WSEventManagerInterface
		Logger                     *zerolog.Logger
		MetadataProviderRef        *util.Ref[metadata_provider.Provider]
		ContinuityManager          *continuity.Manager
		DiscordPresence            *discordrpc_presence.Presence
		PlatformRef                *util.Ref[platform.Platform]
		RefreshAnimeCollectionFunc func()
		IsOfflineRef               *util.Ref[bool]
	}
)

func New(opts NewMpvCoreOptions) *MpvCore {
	return &MpvCore{
		wsEventManager:             opts.WsEventManager,
		continuityManager:          opts.ContinuityManager,
		metadataProviderRef:        opts.MetadataProviderRef,
		discordPresence:            opts.DiscordPresence,
		platformRef:                opts.PlatformRef,
		refreshAnimeCollectionFunc: opts.RefreshAnimeCollectionFunc,
		isOfflineRef:               opts.IsOfflineRef,
	}
}
