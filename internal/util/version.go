package util

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// IsValidBasicSemver
// e.g. "1.2.3" but not "1.2.3-beta" or "1.2"
func IsValidBasicSemver(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}

	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}

	return true
}

// forkPrereleaseID is the pre-release identifier used by fork builds,
// e.g. "3.8.7-fork.3". A fork build keeps its base version (major.minor.patch)
// in sync with the upstream release it tracks and layers additional changes on
// top, incrementing only the fork counter.
const forkPrereleaseID = "fork"

// isForkVersion reports whether v is a fork build, i.e. its pre-release begins
// with the "fork" identifier (e.g. "3.8.7-fork.3").
func isForkVersion(v *semver.Version) bool {
	pr := v.Prerelease()
	if pr == "" {
		return false
	}
	return strings.Split(pr, ".")[0] == forkPrereleaseID
}

// CompareVersion compares two versions and returns the difference between them.
//
//	 3: Current version is newer by major version.
//	 2: Current version is newer by minor version.
//	 1: Current version is newer by patch version.
//		-3: Current version is older by major version.
//		-2: Current version is older by minor version.
//		-1: Current version is older by patch version.
//
// A local fork build (e.g. "3.8.7-fork.3") shares its base version with the
// upstream release it tracks. Standard SemVer ranks a pre-release below its base,
// which would make the app treat the plain upstream release as a newer "update"
// (a downgrade onto upstream). To avoid that, when the current version is a fork
// build with the same base numbers as a plain (non-pre-release) other version, the
// fork is treated as ahead and no update is reported. A genuinely newer upstream
// base (higher major/minor/patch) still wins, and fork-vs-fork comparisons fall
// through to normal SemVer precedence on the fork counter.
func CompareVersion(current string, b string) (comp int, shouldUpdate bool) {

	currV, err := semver.NewVersion(current)
	if err != nil {
		return 0, false
	}
	otherV, err := semver.NewVersion(b)
	if err != nil {
		return 0, false
	}

	if isForkVersion(currV) && otherV.Prerelease() == "" &&
		currV.Major() == otherV.Major() && currV.Minor() == otherV.Minor() && currV.Patch() == otherV.Patch() {
		return 1, false
	}

	comp = currV.Compare(otherV)
	if comp == 0 {
		return 0, false
	}

	if currV.GreaterThan(otherV) {
		shouldUpdate = false

		if currV.Major() > otherV.Major() {
			comp *= 3
		} else if currV.Minor() > otherV.Minor() {
			comp *= 2
		} else if currV.Patch() > otherV.Patch() {
			comp *= 1
		}
	} else if currV.LessThan(otherV) {
		shouldUpdate = true

		if currV.Major() < otherV.Major() {
			comp *= 3
		} else if currV.Minor() < otherV.Minor() {
			comp *= 2
		} else if currV.Patch() < otherV.Patch() {
			comp *= 1
		}
	}

	return comp, shouldUpdate
}

func VersionIsOlderThan(version string, compare string) bool {
	comp, shouldUpdate := CompareVersion(version, compare)
	// shouldUpdate is false means the current version is newer
	return comp < 0 && shouldUpdate
}

var allowedGitHubOwners = []string{"5rahim"}

// validateReleaseUrl checks that the URL points to a GitHub release asset
// from an allowed owner.
func ValidateReleaseUrl(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL")
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed")
	}

	switch parsed.Host {
	case "github.com":
		// e.g. https://github.com/5rahim/seanime/releases/download/v1.0.0/file.zip
		parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
		if len(parts) < 6 || parts[2] != "releases" || parts[3] != "download" {
			return fmt.Errorf("URL must point to a GitHub release asset")
		}
		owner := parts[0]
		for _, allowed := range allowedGitHubOwners {
			if strings.EqualFold(owner, allowed) {
				return nil
			}
		}
		return fmt.Errorf("repository owner %q is not allowed", owner)

	case "seanime.app":
		return nil

	default:
		return fmt.Errorf("host %q is not allowed", parsed.Host)
	}
}
