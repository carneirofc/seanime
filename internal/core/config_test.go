package core

import (
	"os"
	"path/filepath"
	"runtime"
	"seanime/internal/constants"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// These tests exercise the path resolution helpers directly rather than NewConfig, which
// mutates the global viper singleton and writes files.

func TestResolveCallerPathAbsoluteIsCleanedNotRewritten(t *testing.T) {
	dir := t.TempDir()
	messy := filepath.Join(dir, "sub", "..", "data")

	got, err := resolveCallerPath(messy)
	if err != nil {
		t.Fatalf("resolveCallerPath(%q) returned error: %v", messy, err)
	}
	if want := filepath.Join(dir, "data"); got != want {
		t.Fatalf("resolveCallerPath(%q) = %q, want %q", messy, got, want)
	}
}

func TestResolveCallerPathRelativeUsesWorkingDirectory(t *testing.T) {
	// The point of the feature: `--datadir=./dev-datadir` from the repo root must land
	// beside the caller, not be rejected.
	wd := t.TempDir()
	t.Chdir(wd)

	cases := map[string]string{
		"dev-datadir":     filepath.Join(wd, "dev-datadir"),
		"./dev-datadir":   filepath.Join(wd, "dev-datadir"),
		"sub/dir":         filepath.Join(wd, "sub", "dir"),
		" dev-datadir ":   filepath.Join(wd, "dev-datadir"),
		"../sibling-data": filepath.Join(filepath.Dir(wd), "sibling-data"),
	}

	for in, want := range cases {
		got, err := resolveCallerPath(in)
		if err != nil {
			t.Fatalf("resolveCallerPath(%q) returned error: %v", in, err)
		}
		if got != want {
			t.Errorf("resolveCallerPath(%q) = %q, want %q", in, got, want)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("resolveCallerPath(%q) = %q, which is not absolute", in, got)
		}
	}
}

func TestResolveCallerPathExpandsEnvironmentVariables(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SEANIME_TEST_DATA_ROOT", dir)

	got, err := resolveCallerPath("$SEANIME_TEST_DATA_ROOT/instance-1")
	if err != nil {
		t.Fatalf("resolveCallerPath returned error: %v", err)
	}
	if want := filepath.Join(dir, "instance-1"); got != want {
		t.Fatalf("resolveCallerPath = %q, want %q", got, want)
	}
}

func TestResolveCallerPathRejectsPathsThatNameNoDirectory(t *testing.T) {
	// filepath.Abs("") and filepath.Abs(".") both return the working directory, so
	// accepting these would scatter config.toml, database.db, logs/ and cache/ over
	// whatever directory the process happened to be started from.
	t.Chdir(t.TempDir())
	os.Unsetenv("SEANIME_TEST_UNSET_VAR")

	for _, in := range []string{"", "  ", ".", "./", "sub/..", "$SEANIME_TEST_UNSET_VAR"} {
		got, err := resolveCallerPath(in)
		if err == nil {
			t.Errorf("resolveCallerPath(%q) = %q, want an error", in, got)
		}
	}
}

func TestResolveConfigPathAnchorsRelativeValuesToTheDataDirectory(t *testing.T) {
	// config.toml lives inside the data directory, so a hand-written relative value there
	// means "next to the rest of the data" — never the launch directory, which differs
	// between the installer, the systray and a systemd unit.
	dataDir := t.TempDir()
	t.Chdir(t.TempDir())

	cases := map[string]string{
		"cache":                        filepath.Join(dataDir, "cache"),
		"./cache":                      filepath.Join(dataDir, "cache"),
		"cache/transcode":              filepath.Join(dataDir, "cache", "transcode"),
		filepath.Join(dataDir, "logs"): filepath.Join(dataDir, "logs"),
	}

	for in, want := range cases {
		if got := resolveConfigPath(in, dataDir); got != want {
			t.Errorf("resolveConfigPath(%q, %q) = %q, want %q", in, dataDir, got, want)
		}
	}
}

func TestResolveConfigPathExpandsDataDirPlaceholder(t *testing.T) {
	// The shipped defaults are literal "$SEANIME_DATA_DIR/<subdir>" strings, which is what
	// keeps a data directory relocatable.
	dataDir := t.TempDir()
	t.Setenv("SEANIME_DATA_DIR", dataDir)

	if got, want := resolveConfigPath("$SEANIME_DATA_DIR/cache", dataDir), filepath.Join(dataDir, "cache"); got != want {
		t.Fatalf("resolveConfigPath = %q, want %q", got, want)
	}
}

func TestResolveConfigPathKeepsEmptyValuesEmpty(t *testing.T) {
	// validateConfig reports these with its own "cannot be empty" message; turning an
	// empty value into the working directory would hide that.
	os.Unsetenv("SEANIME_TEST_UNSET_VAR")

	for _, in := range []string{"", "$SEANIME_TEST_UNSET_VAR"} {
		if got := resolveConfigPath(in, t.TempDir()); got != "" {
			t.Errorf("resolveConfigPath(%q, ...) = %q, want an empty string", in, got)
		}
	}
}

func TestResolveConfigPathWithoutBaseFallsBackToWorkingDirectory(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)

	if got, want := resolveConfigPath("cache", ""), filepath.Join(wd, "cache"); got != want {
		t.Fatalf("resolveConfigPath(\"cache\", \"\") = %q, want %q", got, want)
	}
}

func TestExpandEnvironmentValuesResolvesEveryPathField(t *testing.T) {
	// Every path field must come out absolute, including offline.dir, offline.assetDir and
	// torrent.dir — validateConfig never checked those, but they feed the writable-roots
	// confinement list.
	dataDir := t.TempDir()
	t.Chdir(t.TempDir())

	cfg := &Config{}
	cfg.Web.AssetDir = "assets"
	cfg.Cache.Dir = "cache"
	cfg.Cache.TranscodeDir = "cache/transcode"
	cfg.Logs.Dir = "logs"
	cfg.Manga.DownloadDir = "manga"
	cfg.Manga.LocalDir = "manga-local"
	cfg.Offline.Dir = "offline"
	cfg.Offline.AssetDir = "offline/assets"
	cfg.Extensions.Dir = "extensions"
	cfg.Torrent.Dir = "torrent"
	cfg.Server.Tls.CertPath = "certs/cert.pem"
	cfg.Server.Tls.KeyPath = "certs/key.pem"

	expandEnvironmentValues(cfg, dataDir)

	resolved := map[string]string{
		"web.assetDir":        cfg.Web.AssetDir,
		"cache.dir":           cfg.Cache.Dir,
		"cache.transcodeDir":  cfg.Cache.TranscodeDir,
		"logs.dir":            cfg.Logs.Dir,
		"manga.downloadDir":   cfg.Manga.DownloadDir,
		"manga.localDir":      cfg.Manga.LocalDir,
		"offline.dir":         cfg.Offline.Dir,
		"offline.assetDir":    cfg.Offline.AssetDir,
		"extensions.dir":      cfg.Extensions.Dir,
		"torrent.dir":         cfg.Torrent.Dir,
		"server.tls.certPath": cfg.Server.Tls.CertPath,
		"server.tls.keyPath":  cfg.Server.Tls.KeyPath,
	}

	for key, path := range resolved {
		if err := checkIsValidPath(path); err != nil {
			t.Errorf("%s = %q: %v", key, path, err)
		}
		if !strings.HasPrefix(path, dataDir+string(os.PathSeparator)) {
			t.Errorf("%s = %q, want it under the data directory %q", key, path, dataDir)
		}
	}
}

func TestExpandEnvironmentValuesSatisfiesValidateConfig(t *testing.T) {
	// The fields validateConfig checks must all survive a round trip from relative values.
	dataDir := t.TempDir()
	t.Chdir(t.TempDir())
	logger := zerolog.Nop()

	cfg := &Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 43211
	cfg.Database.Name = "seanime"
	cfg.Web.AssetDir = "assets"
	cfg.Cache.Dir = "cache"
	cfg.Cache.TranscodeDir = "cache/transcode"
	cfg.Logs.Dir = "logs"
	cfg.Manga.DownloadDir = "manga"
	cfg.Manga.LocalDir = "manga-local"
	cfg.Extensions.Dir = "extensions"
	cfg.Server.Tls.Enabled = true
	cfg.Server.Tls.CertPath = "certs/cert.pem"
	cfg.Server.Tls.KeyPath = "certs/key.pem"

	expandEnvironmentValues(cfg, dataDir)

	if err := validateConfig(cfg, &logger); err != nil {
		t.Fatalf("validateConfig after resolution returned error: %v", err)
	}
}

func TestInitAppDataDirAcceptsRelativePath(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	logger := zerolog.Nop()

	dataDir, configPath, err := initAppDataDir("./dev-datadir", &logger)
	if err != nil {
		t.Fatalf("initAppDataDir returned error: %v", err)
	}

	if want := filepath.Join(wd, "dev-datadir"); dataDir != want {
		t.Fatalf("dataDir = %q, want %q", dataDir, want)
	}
	if want := filepath.Join(dataDir, constants.ConfigFileName); configPath != want {
		t.Fatalf("configPath = %q, want %q", configPath, want)
	}
	if info, statErr := os.Stat(dataDir); statErr != nil || !info.IsDir() {
		t.Fatalf("data directory %q was not created: %v", dataDir, statErr)
	}
}

func TestInitAppDataDirRejectsPathsThatNameNoDirectory(t *testing.T) {
	// Regression guard: the data directory must never silently become the caller's cwd.
	wd := t.TempDir()
	t.Chdir(wd)
	os.Unsetenv("SEANIME_TEST_UNSET_VAR")
	logger := zerolog.Nop()

	if _, _, err := initAppDataDir("$SEANIME_TEST_UNSET_VAR", &logger); err == nil {
		t.Fatal("initAppDataDir(\"$SEANIME_TEST_UNSET_VAR\") succeeded, want an error")
	}
	if _, err := os.Stat(filepath.Join(wd, constants.ConfigFileName)); !os.IsNotExist(err) {
		t.Fatalf("a config file was created in the working directory: %v", err)
	}
}

func TestInitAppDataDirDefaultsToTheOsConfigDirectory(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("os.UserConfigDir is only steerable through XDG_CONFIG_HOME on linux")
	}
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	logger := zerolog.Nop()

	dataDir, _, err := initAppDataDir("", &logger)
	if err != nil {
		t.Fatalf("initAppDataDir(\"\") returned error: %v", err)
	}
	if want := filepath.Join(configHome, "Seanime"); dataDir != want {
		t.Fatalf("dataDir = %q, want %q", dataDir, want)
	}
}

func TestSetDataDirEnvOverwritesARelativeValue(t *testing.T) {
	// Without this, a relative SEANIME_DATA_DIR survives into every
	// "$SEANIME_DATA_DIR/<subdir>" default and makes the whole config relative.
	dataDir := t.TempDir()
	t.Setenv("SEANIME_DATA_DIR", "./dev-datadir")

	if err := setDataDirEnv(dataDir); err != nil {
		t.Fatalf("setDataDirEnv returned error: %v", err)
	}
	if got := os.Getenv("SEANIME_DATA_DIR"); got != dataDir {
		t.Fatalf("SEANIME_DATA_DIR = %q, want %q", got, dataDir)
	}
}
