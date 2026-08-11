package security

import (
	"slices"
	"sort"
	"strings"
)

// Privileged capabilities.
//
// These gate the actions that let an API caller reach past the media library and
// touch the host: spawning processes, walking the filesystem, loading third-party
// code, replacing the running binary. None of them is inferable from a request —
// not from its source address, not from its headers, not from an authenticated
// session. A session proves the caller is on the allowlist; it does not prove they
// should be able to execute code on the machine.
//
// The set is therefore operator configuration (`server.capabilities`) and is
// deliberately NOT settable through the API: a capability the API can grant itself
// is not a capability gate.
const (
	// CapabilityFilesystem covers host filesystem browsing, arbitrary read/write
	// paths, and mutating the configured library/download roots.
	CapabilityFilesystem = "filesystem"
	// CapabilityExec covers spawning local processes: media players, torrent
	// clients, ffmpeg/ffprobe, the file explorer, and plugin $os.cmd.
	CapabilityExec = "exec"
	// CapabilityExtensions covers installing, updating and reloading third-party
	// extensions, including from remote URLs and git repositories.
	CapabilityExtensions = "extensions"
	// CapabilitySelfUpdate covers downloading a release and replacing the running
	// binary.
	CapabilitySelfUpdate = "selfupdate"
	// CapabilityNakamaHost covers serving the peer-to-peer Nakama host endpoints,
	// which authenticate with their own shared password instead of the session.
	CapabilityNakamaHost = "nakama-host"

	// capabilityAll and capabilityNone are explicit shorthands for the config file.
	capabilityAll  = "all"
	capabilityNone = "none"
)

// AllCapabilities is the full set, in a stable order for logging.
var AllCapabilities = []string{
	CapabilityExec,
	CapabilityExtensions,
	CapabilityFilesystem,
	CapabilityNakamaHost,
	CapabilitySelfUpdate,
}

// ParseCapabilities normalizes a configured capability list. Unknown entries are
// returned separately so the caller can warn about them rather than silently
// granting or dropping something the operator thought they had configured.
func ParseCapabilities(raw []string) (capabilities []string, unknown []string) {
	capabilities = make([]string, 0, len(raw))

	for _, entry := range raw {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}

		switch {
		case entry == capabilityAll:
			capabilities = append(capabilities, AllCapabilities...)
		case entry == capabilityNone:
			// Explicit deny-all; contributes nothing.
		case slices.Contains(AllCapabilities, entry):
			capabilities = append(capabilities, entry)
		default:
			unknown = append(unknown, entry)
		}
	}

	sort.Strings(capabilities)
	return slices.Compact(capabilities), unknown
}

// SetCapabilities installs the effective capability set. configured reports
// whether the operator supplied `server.capabilities` at all: an explicitly empty
// list denies everything, whereas an absent one falls back to the posture default
// resolved by ResolveDefaultCapabilities.
func SetCapabilities(capabilities []string, configured bool) {
	current := GlobalSecurityContext.Get()
	next := cloneSecurityContext(current)
	next.Capabilities = slices.Clone(capabilities)
	next.CapabilitiesConfigured = configured
	GlobalSecurityContext.Set(next)
}

// ResolveDefaultCapabilities is the fallback when `server.capabilities` is absent.
//
// A server deployment — anything fronted by a proxy, reachable at a canonical
// public URL, or gating access behind an identity provider — starts with nothing
// granted; the operator opts in per capability. A plain local install, where the
// only client is the person at the keyboard and there is no network boundary to
// speak of, keeps the historical behaviour and gets everything.
//
// This is a deployment-wide default, not a per-request decision: once resolved,
// every caller is treated identically.
func ResolveDefaultCapabilities(oidcEnabled bool, externalURL string, trustedProxies []string) []string {
	isServerDeployment := oidcEnabled ||
		strings.TrimSpace(externalURL) != "" ||
		len(trustedProxies) > 0

	if isServerDeployment {
		return nil
	}

	return slices.Clone(AllCapabilities)
}

// Allows reports whether a privileged capability is granted.
func Allows(capability string) bool {
	return slices.Contains(GlobalSecurityContext.Get().Capabilities, capability)
}

// GetCapabilities returns the effective capability set, for logging and tests.
func GetCapabilities() []string {
	return slices.Clone(GlobalSecurityContext.Get().Capabilities)
}
