package push

import "os"

// Environment variables that configure push. Push is enabled only when all
// three VAPID values are present; the store path has a sensible default.
const (
	EnvPublicKey  = "OFFICE_VAPID_PUBLIC_KEY"
	EnvPrivateKey = "OFFICE_VAPID_PRIVATE_KEY"
	EnvSubject    = "OFFICE_VAPID_SUBJECT"
	EnvStorePath  = "OFFICE_PUSH_STORE"
)

// DefaultStorePath is where subscriptions are written when EnvStorePath is
// unset. On a single-instance deployment this should sit on a persistent
// volume so subscriptions survive restarts.
const DefaultStorePath = "push-subscriptions.json"

// ConfigFromEnv reads the VAPID configuration from the environment. ok is
// false (with no error) when push is not configured, which is the normal state
// for local development and the current production default — the server then
// runs without push rather than failing to start.
func ConfigFromEnv() (cfg Config, ok bool) {
	cfg = Config{
		PublicKey:  os.Getenv(EnvPublicKey),
		PrivateKey: os.Getenv(EnvPrivateKey),
		Subject:    os.Getenv(EnvSubject),
	}
	if cfg.PublicKey == "" || cfg.PrivateKey == "" || cfg.Subject == "" {
		return Config{}, false
	}
	return cfg, true
}

// StorePathFromEnv returns the subscription store path, honoring EnvStorePath
// and falling back to DefaultStorePath.
func StorePathFromEnv() string {
	if p := os.Getenv(EnvStorePath); p != "" {
		return p
	}
	return DefaultStorePath
}
