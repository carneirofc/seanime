package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"seanime/internal/constants"
	"seanime/internal/util"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Config struct {
	Version string
	Server  struct {
		Host            string
		Port            int
		Offline         bool
		UseBinaryPath   bool // Makes $SEANIME_WORKING_DIR point to the binary's directory
		Systray         bool
		DoHUrl          string
		Password        string
		SecureMode      string   // empty = current baseline defaults, "hardened" opts into a stricter passwordless boundary, "lax" disables baseline request-boundary restrictions, "strict" includes hardened plus extra restrictions
		AccessAllowlist []string // Additional remote hosts/origins allowed through the passwordless API/events boundary
		TrustedProxies  []string // Explicit reverse proxies allowed to supply forwarded client IP/host/proto headers
		ExternalURL     string   // Canonical public URL used for proxy-aware secure cookies and request normalization
		// Capabilities grants privileged actions that let a caller reach past the
		// media library and touch the host: "exec", "filesystem", "extensions",
		// "selfupdate", "nakama-host" (or "all" / "none"). Never derived from a
		// request and never settable through the API. Absent means "use the posture
		// default": nothing for a server deployment, everything for a local install.
		Capabilities []string
		Tls          struct {
			Enabled  bool
			CertPath string
			KeyPath  string
		}
		Oidc struct {
			IssuerURL        string // OIDC issuer; discovery happens at <issuer>/.well-known/openid-configuration
			ClientID         string
			ClientSecret     string   // can be supplied via SEANIME_OIDC_CLIENT_SECRET instead of the config file
			Scopes           []string // defaults to ["openid", "profile", "email"]
			UsernameClaim    string   // ID-token claim matched against AllowedUsernames, defaults to "preferred_username"
			AllowedSubjects  []string // stable `sub` claim values; survives username changes at the IdP
			AllowedUsernames []string // matched case-insensitively against UsernameClaim (falls back to "email")
			ProviderName     string   // display name for the login button, defaults to "SSO"
			SessionTTLDays   int      // sliding session expiry, defaults to 30
			SessionMaxDays   int      // absolute session lifetime cap, defaults to 90
			AllowInsecure    bool     // dev only: permits http ExternalURL and non-__Host- cookies
		}
	}
	Database struct {
		Name string
	}
	Web struct {
		AssetDir string
	}
	Logs struct {
		Dir string
	}
	Cache struct {
		Dir          string
		TranscodeDir string
	}
	Offline struct {
		Dir      string
		AssetDir string
	}
	Manga struct {
		DownloadDir string
		LocalDir    string
	}
	Data struct { // Hydrated after config is loaded
		AppDataDir string
		WorkingDir string
	}
	Extensions struct {
		Dir string
	}
	Torrent struct {
		Dir string
	}
	Anilist struct {
		ClientID string
	}
	Experimental struct {
		BuiltinTorrentClient bool
		// MCP exposes a read-only Model Context Protocol server at /api/v1/mcp.
		MCP         bool
		DummyDebrid bool
	}
}

type ConfigOptions struct {
	Flags           SeanimeFlags
	OnVersionChange []func(oldVersion string, newVersion string)
	EmbeddedLogo    []byte // The embedded logo
}

// NewConfig initializes the config
func NewConfig(options *ConfigOptions, logger *zerolog.Logger) (*Config, error) {
	flags := options.Flags

	logger.Debug().Msg("app: Initializing config")

	definedDataDir := ""

	// Set data dir (flag overrides env var)
	if os.Getenv("SEANIME_DATA_DIR") != "" {
		definedDataDir = os.Getenv("SEANIME_DATA_DIR")
	}

	if flags.DataDir != "" {
		definedDataDir = flags.DataDir
	}

	defaultHost := "127.0.0.1"
	defaultPort := 43211

	// Environment variables override defaults
	if os.Getenv("SEANIME_SERVER_HOST") != "" {
		defaultHost = os.Getenv("SEANIME_SERVER_HOST")
	}
	if os.Getenv("SEANIME_SERVER_PORT") != "" {
		var err error
		defaultPort, err = strconv.Atoi(os.Getenv("SEANIME_SERVER_PORT"))
		if err != nil {
			return nil, fmt.Errorf("invalid SEANIME_SERVER_PORT environment variable: %s", os.Getenv("SEANIME_SERVER_PORT"))
		}
	}

	// Flags override environment variables
	if flags.Host != "" {
		defaultHost = flags.Host
	}
	if flags.Port != 0 {
		defaultPort = flags.Port
	}

	// Initialize the app data directory
	dataDir, configPath, err := initAppDataDir(definedDataDir, logger)
	if err != nil {
		return nil, err
	}

	// Create assets directory if it doesn't exist
	_ = os.MkdirAll(filepath.Join(dataDir, "assets"), 0700)

	// Set Seanime's default custom environment variables
	if err = setDataDirEnv(dataDir); err != nil {
		return nil, err
	}

	// Configure viper
	viper.SetConfigName(constants.ConfigFileName)
	viper.SetConfigType("toml")
	viper.SetConfigFile(configPath)

	// Set default values
	viper.SetDefault("version", constants.Version)
	viper.SetDefault("server.host", defaultHost)
	viper.SetDefault("server.port", defaultPort)
	viper.SetDefault("server.offline", false)
	//viper.SetDefault("server.secureMode", "")
	//viper.SetDefault("server.accessAllowlist", []string{})
	//viper.SetDefault("server.trustedProxies", []string{})
	//viper.SetDefault("server.externalURL", "")
	// Use the binary's directory as the working directory environment variable on macOS
	viper.SetDefault("server.useBinaryPath", true)
	// viper.SetDefault("server.systray", true)
	viper.SetDefault("database.name", "seanime")
	viper.SetDefault("web.assetDir", "$SEANIME_DATA_DIR/assets")
	viper.SetDefault("cache.dir", "$SEANIME_DATA_DIR/cache")
	viper.SetDefault("cache.transcodeDir", "$SEANIME_DATA_DIR/cache/transcode")
	viper.SetDefault("manga.downloadDir", "$SEANIME_DATA_DIR/manga")
	viper.SetDefault("manga.localDir", "$SEANIME_DATA_DIR/manga-local")
	viper.SetDefault("logs.dir", "$SEANIME_DATA_DIR/logs")
	viper.SetDefault("offline.dir", "$SEANIME_DATA_DIR/offline")
	viper.SetDefault("offline.assetDir", "$SEANIME_DATA_DIR/offline/assets")
	viper.SetDefault("extensions.dir", "$SEANIME_DATA_DIR/extensions")
	viper.SetDefault("torrent.dir", "$SEANIME_DATA_DIR/torrent")
	viper.SetDefault("server.oidc.scopes", []string{"openid", "profile", "email"})
	viper.SetDefault("server.oidc.usernameClaim", "preferred_username")
	viper.SetDefault("server.oidc.providerName", "SSO")
	viper.SetDefault("server.oidc.sessionTTLDays", 30)
	viper.SetDefault("server.oidc.sessionMaxDays", 90)
	// Allow the OIDC client secret to be supplied via the environment instead of the config file
	_ = viper.BindEnv("server.oidc.clientSecret", "SEANIME_OIDC_CLIENT_SECRET")

	// Create and populate the config file if it doesn't exist
	if err = createConfigFile(configPath); err != nil {
		return nil, err
	}

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	// Check if host or port have been overridden and differ from config file
	existingHost := viper.GetString("server.host")
	existingPort := viper.GetInt("server.port")
	hostChanged := false
	portChanged := false

	if (flags.Host != "" || os.Getenv("SEANIME_SERVER_HOST") != "") && existingHost != defaultHost {
		viper.Set("server.host", defaultHost)
		hostChanged = true
	}
	if (flags.Port != 0 || os.Getenv("SEANIME_SERVER_PORT") != "") && existingPort != defaultPort {
		viper.Set("server.port", defaultPort)
		portChanged = true
	}
	if flags.Password != "" {
		viper.Set("server.password", flags.Password)
		logger.Info().Msg("app: Set server password")
	}
	if flags.DisablePassword {
		viper.Set("server.password", "")
		logger.Info().Msg("app: Disabled server password")
	}

	// Write config if host or port changed
	if hostChanged || portChanged {
		if err := viper.WriteConfig(); err != nil {
			logger.Warn().Err(err).Msg("app: Failed to update config with new host/port")
		} else {
			logger.Info().
				Bool("hostChanged", hostChanged).
				Bool("portChanged", portChanged).
				Str("host", defaultHost).
				Int("port", defaultPort).
				Msg("app: Updated config with new host/port")
		}
	}

	// Unmarshal the config values
	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	// Update the config if the version has changed
	if err := updateVersion(cfg, options); err != nil {
		return nil, err
	}

	// Before expanding the values, check if we need to override the working directory
	if err = setWorkingDirEnv(cfg.Server.UseBinaryPath); err != nil {
		return nil, err
	}

	// Expand the values, replacing environment variables
	expandEnvironmentValues(cfg)
	cfg.Data.AppDataDir = dataDir
	cfg.Data.WorkingDir = os.Getenv("SEANIME_WORKING_DIR")

	if cfg.Server.Tls.Enabled && (cfg.Server.Tls.CertPath == "" || cfg.Server.Tls.KeyPath == "") {
		viper.SetDefault("server.tls.certPath", "$SEANIME_DATA_DIR/certs/cert.pem")
		viper.SetDefault("server.tls.keyPath", "$SEANIME_DATA_DIR/certs/key.pem")
		_ = viper.WriteConfig()
		_ = viper.ReadInConfig()
		_ = viper.Unmarshal(cfg)
		expandEnvironmentValues(cfg)
	}

	// Check validity of the config
	if err := validateConfig(cfg, logger); err != nil {
		return nil, err
	}

	go loadLogo(options.EmbeddedLogo, dataDir)

	return cfg, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// IsOidcMode reports whether OIDC login is configured. While OIDC is active the
// server password is ignored entirely and browser access requires an IdP session.
func (cfg *Config) IsOidcMode() bool {
	return cfg.Server.Oidc.IssuerURL != "" && cfg.Server.Oidc.ClientID != "" && cfg.Server.Oidc.ClientSecret != ""
}

func (cfg *Config) GetServerAddr(df ...string) string {
	return fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
}

func (cfg *Config) GetServerURI(df ...string) string {
	scheme := "http"
	if cfg.Server.Tls.Enabled {
		scheme = "https"
	}
	pAddr := fmt.Sprintf("%s://%s", scheme, cfg.GetServerAddr(df...))
	if cfg.Server.Host == "" || cfg.Server.Host == "0.0.0.0" {
		pAddr = fmt.Sprintf(":%d", cfg.Server.Port)
		if len(df) > 0 {
			pAddr = fmt.Sprintf("%s://%s:%d", scheme, df[0], cfg.Server.Port)
		}
	}
	return pAddr
}

func getWorkingDir(useBinaryPath bool) (string, error) {
	// Get the working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	binaryDir := ""
	if exe, err := os.Executable(); err == nil {
		if p, err := filepath.EvalSymlinks(exe); err == nil {
			binaryDir = filepath.Dir(p)
			binaryDir = filepath.FromSlash(binaryDir)
		}
	}

	if useBinaryPath && binaryDir != "" {
		return binaryDir, nil
	}

	//// Use the binary's directory as the working directory if needed
	//if useBinaryPath {
	//	exe, err := os.Executable()
	//	if err != nil {
	//		return wd, nil // Fallback to working dir
	//	}
	//	p, err := filepath.EvalSymlinks(exe)
	//	if err != nil {
	//		return wd, nil // Fallback to working dir
	//	}
	//	wd = filepath.Dir(p) // Set the binary's directory as the working directory
	//	return wd, nil
	//}
	return wd, nil
}

func setDataDirEnv(dataDir string) error {
	// Set the data directory environment variable
	if os.Getenv("SEANIME_DATA_DIR") == "" {
		if err := os.Setenv("SEANIME_DATA_DIR", dataDir); err != nil {
			return err
		}
	}

	return nil
}

func setWorkingDirEnv(useBinaryPath bool) error {
	// Set the working directory environment variable
	wd, err := getWorkingDir(useBinaryPath)
	if err != nil {
		return err
	}
	if err = os.Setenv("SEANIME_WORKING_DIR", filepath.FromSlash(wd)); err != nil {
		return err
	}

	return nil
}

// validateConfig checks if the config values are valid
func validateConfig(cfg *Config, logger *zerolog.Logger) error {
	if cfg.Server.Host == "" {
		return errInvalidConfigValue("server.host", "cannot be empty")
	}
	if cfg.Server.Port == 0 {
		return errInvalidConfigValue("server.port", "cannot be 0")
	}
	if cfg.Database.Name == "" {
		return errInvalidConfigValue("database.name", "cannot be empty")
	}
	if cfg.Web.AssetDir == "" {
		return errInvalidConfigValue("web.assetDir", "cannot be empty")
	}
	if err := checkIsValidPath(cfg.Web.AssetDir); err != nil {
		return wrapInvalidConfigValue("web.assetDir", err)
	}

	if cfg.Cache.Dir == "" {
		return errInvalidConfigValue("cache.dir", "cannot be empty")
	}
	if err := checkIsValidPath(cfg.Cache.Dir); err != nil {
		return wrapInvalidConfigValue("cache.dir", err)
	}

	if cfg.Cache.TranscodeDir == "" {
		return errInvalidConfigValue("cache.transcodeDir", "cannot be empty")
	}
	if err := checkIsValidPath(cfg.Cache.TranscodeDir); err != nil {
		return wrapInvalidConfigValue("cache.transcodeDir", err)
	}

	if cfg.Logs.Dir == "" {
		return errInvalidConfigValue("logs.dir", "cannot be empty")
	}
	if err := checkIsValidPath(cfg.Logs.Dir); err != nil {
		return wrapInvalidConfigValue("logs.dir", err)
	}

	if cfg.Manga.DownloadDir == "" {
		return errInvalidConfigValue("manga.downloadDir", "cannot be empty")
	}
	if err := checkIsValidPath(cfg.Manga.DownloadDir); err != nil {
		return wrapInvalidConfigValue("manga.downloadDir", err)
	}

	if cfg.Manga.LocalDir == "" {
		return errInvalidConfigValue("manga.localDir", "cannot be empty")
	}
	if err := checkIsValidPath(cfg.Manga.LocalDir); err != nil {
		return wrapInvalidConfigValue("manga.localDir", err)
	}

	if cfg.Extensions.Dir == "" {
		return errInvalidConfigValue("extensions.dir", "cannot be empty")
	}
	if err := checkIsValidPath(cfg.Extensions.Dir); err != nil {
		return wrapInvalidConfigValue("extensions.dir", err)
	}

	if oidc := cfg.Server.Oidc; oidc.IssuerURL != "" || oidc.ClientID != "" || oidc.ClientSecret != "" {
		if !cfg.IsOidcMode() {
			return errInvalidConfigValue("server.oidc", "issuerURL, clientID and clientSecret must all be set to enable OIDC login")
		}
		if cfg.Server.ExternalURL == "" {
			return errInvalidConfigValue("server.externalURL", "must be set when OIDC login is enabled (used to derive the redirect URI)")
		}
		if !oidc.AllowInsecure && !strings.HasPrefix(cfg.Server.ExternalURL, "https://") {
			return errInvalidConfigValue("server.externalURL", "must use https:// when OIDC login is enabled (set server.oidc.allowInsecure for local development)")
		}
		if len(oidc.AllowedSubjects) == 0 && len(oidc.AllowedUsernames) == 0 {
			return errInvalidConfigValue("server.oidc", "at least one of allowedSubjects or allowedUsernames must be set; refusing to admit every IdP account")
		}
		if oidc.SessionTTLDays <= 0 {
			return errInvalidConfigValue("server.oidc.sessionTTLDays", "must be positive")
		}
		if oidc.SessionMaxDays < oidc.SessionTTLDays {
			return errInvalidConfigValue("server.oidc.sessionMaxDays", "cannot be lower than sessionTTLDays")
		}
	}

	if cfg.Server.Tls.Enabled {
		if cfg.Server.Tls.CertPath == "" {
			return errInvalidConfigValue("server.tls.certPath", "cannot be empty when TLS is enabled")
		}
		if err := checkIsValidPath(cfg.Server.Tls.CertPath); err != nil {
			return wrapInvalidConfigValue("server.tls.certPath", err)
		}
		if cfg.Server.Tls.KeyPath == "" {
			return errInvalidConfigValue("server.tls.keyPath", "cannot be empty when TLS is enabled")
		}
		if err := checkIsValidPath(cfg.Server.Tls.KeyPath); err != nil {
			return wrapInvalidConfigValue("server.tls.keyPath", err)
		}
	}

	return nil
}

func checkIsValidPath(path string) error {
	ok := filepath.IsAbs(path)
	if !ok {
		return errors.New("path is not an absolute path")
	}
	return nil
}

// errInvalidConfigValue returns an error for an invalid config value
func errInvalidConfigValue(s string, s2 string) error {
	return fmt.Errorf("invalid config value: \"%s\" %s", s, s2)
}
func wrapInvalidConfigValue(s string, err error) error {
	return fmt.Errorf("invalid config value: \"%s\" %w", s, err)
}

func updateVersion(cfg *Config, opts *ConfigOptions) error {
	defer func() {
		if r := recover(); r != nil {
			// Do nothing
		}
	}()

	if cfg.Version != constants.Version {
		for _, f := range opts.OnVersionChange {
			f(cfg.Version, constants.Version)
		}
		cfg.Version = constants.Version
	}

	viper.Set("version", constants.Version)

	return viper.WriteConfig()
}

func expandEnvironmentValues(cfg *Config) {
	defer func() {
		if r := recover(); r != nil {
			// Do nothing
		}
	}()
	cfg.Web.AssetDir = filepath.FromSlash(os.ExpandEnv(cfg.Web.AssetDir))
	cfg.Cache.Dir = filepath.FromSlash(os.ExpandEnv(cfg.Cache.Dir))
	cfg.Cache.TranscodeDir = filepath.FromSlash(os.ExpandEnv(cfg.Cache.TranscodeDir))
	cfg.Logs.Dir = filepath.FromSlash(os.ExpandEnv(cfg.Logs.Dir))
	cfg.Manga.DownloadDir = filepath.FromSlash(os.ExpandEnv(cfg.Manga.DownloadDir))
	cfg.Manga.LocalDir = filepath.FromSlash(os.ExpandEnv(cfg.Manga.LocalDir))
	cfg.Offline.Dir = filepath.FromSlash(os.ExpandEnv(cfg.Offline.Dir))
	cfg.Offline.AssetDir = filepath.FromSlash(os.ExpandEnv(cfg.Offline.AssetDir))
	cfg.Extensions.Dir = filepath.FromSlash(os.ExpandEnv(cfg.Extensions.Dir))
	cfg.Torrent.Dir = filepath.FromSlash(os.ExpandEnv(cfg.Torrent.Dir))
	cfg.Server.Tls.CertPath = filepath.FromSlash(os.ExpandEnv(cfg.Server.Tls.CertPath))
	cfg.Server.Tls.KeyPath = filepath.FromSlash(os.ExpandEnv(cfg.Server.Tls.KeyPath))
}

// createConfigFile creates a default config file if it doesn't exist
func createConfigFile(configPath string) error {
	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
			return err
		}
		if err := viper.WriteConfig(); err != nil {
			return err
		}
	}
	return nil
}

func initAppDataDir(definedDataDir string, logger *zerolog.Logger) (dataDir string, configPath string, err error) {

	// User defined data directory
	if definedDataDir != "" {

		// Expand environment variables
		definedDataDir = filepath.FromSlash(os.ExpandEnv(definedDataDir))

		if !filepath.IsAbs(definedDataDir) {
			return "", "", errors.New("app: Data directory path must be absolute")
		}

		// Replace the default data directory
		dataDir = definedDataDir

		logger.Trace().Str("dataDir", dataDir).Msg("app: Overriding default data directory")
	} else {
		// Default OS data directory
		// windows: %APPDATA%
		// unix: $XDG_CONFIG_HOME or $HOME
		// darwin: $HOME/Library/Application Support
		dataDir, err = os.UserConfigDir()
		if err != nil {
			return "", "", err
		}
		// Get the app directory
		dataDir = filepath.Join(dataDir, "Seanime")
	}

	// Create data dir if it doesn't exist
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", "", err
	}

	// Get the config file path
	// Normalize the config file path
	configPath = filepath.FromSlash(filepath.Join(dataDir, constants.ConfigFileName))
	// Normalize the data directory path
	dataDir = filepath.FromSlash(dataDir)

	return
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func loadLogo(embeddedLogo []byte, dataDir string) (err error) {
	defer util.HandlePanicInModuleWithError("core/loadLogo", &err)

	if len(embeddedLogo) == 0 {
		return nil
	}

	logoPath := filepath.Join(dataDir, "seanime-logo.png")
	if _, err = os.Stat(logoPath); os.IsNotExist(err) {
		if err = os.WriteFile(logoPath, embeddedLogo, 0644); err != nil {
			return err
		}
	}
	return nil
}
