package extension_repo

import (
	"errors"
	"fmt"
	"regexp"
	"seanime/internal/extension"
	"seanime/internal/util"
	"strings"
)

func pluginManifestSanityCheck(ext *extension.Extension) error {
	// Not a plugin, so no need to check
	if ext.Type != extension.TypePlugin {
		return nil
	}

	// Check plugin manifest
	if ext.Plugin == nil {
		return fmt.Errorf("plugin manifest is missing")
	}

	// Check plugin manifest version
	if ext.Plugin.Version == "" {
		return fmt.Errorf("plugin manifest version is missing")
	}

	// Check plugin permissions version
	if ext.Plugin.Version != extension.PluginManifestVersion {
		return fmt.Errorf("unsupported plugin manifest version: %v", ext.Plugin.Version)
	}

	return nil
}

func manifestSanityCheck(ext *extension.Extension) error {
	if ext.ID == "" || ext.Name == "" || ext.Version == "" || ext.Language == "" || ext.Type == "" || ext.Author == "" {
		return fmt.Errorf("extension is missing required fields, ID: %v, Name: %v, Version: %v, Language: %v, Type: %v, Author: %v, Payload: %v",
			ext.ID, ext.Name, ext.Version, ext.Language, ext.Type, ext.Author, len(ext.Payload))
	}

	if ext.Payload == "" && ext.PayloadURI == "" {
		return fmt.Errorf("extension is missing payload and payload URI")
	}

	// Check the ID
	if err := isValidExtensionID(ext.ID); err != nil {
		return err
	}

	// Check name length
	if len(ext.Name) > 50 {
		return fmt.Errorf("extension name is too long")
	}

	// Check author length
	if len(ext.Author) > 25 {
		return fmt.Errorf("extension author is too long")
	}

	if !util.IsValidBasicSemver(ext.Version) {
		return fmt.Errorf("invalid version: %v", ext.Version)
	}

	// Check language
	if !isKnownExtensionLanguage(ext.Language) {
		return fmt.Errorf("unsupported language: %v", ext.Language)
	}

	// Check type
	if !isKnownExtensionType(ext.Type) {
		return fmt.Errorf("unsupported extension type: %v", ext.Type)
	}

	if ext.Type == extension.TypePlugin {
		if err := pluginManifestSanityCheck(ext); err != nil {
			return err
		}
	}

	ext.Lang = strings.ToLower(ext.Lang)

	return nil
}

// isKnownExtensionLanguage reports whether l is a language the app can interpret.
func isKnownExtensionLanguage(l extension.Language) bool {
	return l == extension.LanguageGo ||
		l == extension.LanguageJavascript ||
		l == extension.LanguageTypescript
}

// isKnownExtensionType reports whether t is an area of the app an extension can target.
func isKnownExtensionType(t extension.Type) bool {
	return t == extension.TypeMangaProvider ||
		t == extension.TypeOnlinestreamProvider ||
		t == extension.TypeAnimeTorrentProvider ||
		t == extension.TypeCustomSource ||
		t == extension.TypePlugin
}

// marketplaceEntrySanityCheck checks that a marketplace listing entry carries enough to be
// shown and installed. It is deliberately weaker than manifestSanityCheck: a listing entry
// points at a manifest rather than being one, so it legitimately carries no version and no
// payload — the default marketplace listing has neither. Type and language are required
// because the UI groups entries by type and filters by language; an entry missing either is
// rendered nowhere rather than rendered badly.
func marketplaceEntrySanityCheck(ext *extension.Extension) error {
	if ext == nil {
		return errors.New("entry is empty")
	}

	if err := isValidExtensionID(ext.ID); err != nil {
		return err
	}

	if ext.ManifestURI == "" {
		return errors.New("entry is missing a manifest URI")
	}

	if ext.Name == "" {
		return errors.New("entry is missing a name")
	}

	if len(ext.Name) > 50 {
		return errors.New("entry name is too long")
	}

	if !isKnownExtensionLanguage(ext.Language) {
		return fmt.Errorf("unsupported language: %v", ext.Language)
	}

	if !isKnownExtensionType(ext.Type) {
		return fmt.Errorf("unsupported extension type: %v", ext.Type)
	}

	return nil
}

// extensionSanityCheck checks if the extension has all the required fields in the manifest.
func (r *Repository) extensionSanityCheck(ext *extension.Extension) error {

	if err := manifestSanityCheck(ext); err != nil {
		return err
	}

	// Check that the ID is unique
	if err := r.isUniqueExtensionID(ext.ID); err != nil {
		return err
	}

	return nil
}

// checks if the extension ID is valid
// Note: The ID must start with a letter and contain only alphanumeric characters
// because it can either be used as a package name or appear in a filename
func isValidExtensionID(id string) error {
	if id == "" {
		return errors.New("extension ID is empty")
	}
	if len(id) > 40 {
		return errors.New("extension ID is too long")
	}
	if len(id) < 3 {
		return errors.New("extension ID is too short")
	}

	if !isValidExtensionIDString(id) {
		return errors.New("extension ID contains invalid characters")
	}

	return nil
}

func isValidExtensionIDString(id string) bool {
	// Check if the ID starts with a letter and contains only alphanumeric characters
	re := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9\-]*[a-zA-Z0-9]$`)
	ok := re.MatchString(id)

	if !ok {
		return false
	}
	return true
}

func (r *Repository) isUniqueExtensionID(id string) error {
	// Check if the ID is not a reserved built-in extension ID
	_, found := r.extensionBankRef.Get().Get(id)
	if found {
		return errors.New("extension ID is already in use")
	}
	return nil
}

func ReplacePackageName(src string, newPkgName string) string {
	rgxp, err := regexp.Compile(`package \w+`)
	if err != nil {
		return ""
	}

	ogPkg := rgxp.FindString(src)
	if ogPkg == "" {
		return src
	}

	return strings.Replace(src, ogPkg, "package "+newPkgName, 1)
}
