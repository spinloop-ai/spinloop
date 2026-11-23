package cloud

import "github.com/spinloop-ai/spinloop/internal/config"

// ConfigHome returns spinloop's own configuration directory, where the things
// that are genuinely this machine's live: the stored control-plane credential
// (`cloud-<region>.json`) and the CDK source cache. It delegates to
// internal/config.Dir, so the SPINLOOP_CONFIG_DIR override and the fallback
// rules are resolved in one place; it fails when the directory cannot be
// determined (see config.Dir).
//
// No fact about the cloud belongs here. Which environments exist, which plane
// they are on and what address any of them answers on are the cloud's own
// record, read from the plane; a file naming any of them would be a cache that
// only the machine which wrote it can trust. In particular there is no
// `clouds/<name>/cloud.json` to write, read or migrate.
func ConfigHome() (string, error) {
	return config.Dir()
}
