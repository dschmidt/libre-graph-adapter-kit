// Package config provides configuration-loading helpers shared by LibreGraph
// adapter services: a yaml config-file loader modelled on OpenCloud's own,
// kept dependency-light.
//
// BindSourcesToStructs is extracted and adapted from opencloud-eu/opencloud
// (Apache-2.0), pkg/config.BindSourcesToStructs. It is reproduced here so
// adapter services can load an OpenCloud-style yaml config file WITHOUT
// importing opencloud/pkg/config, which pulls in every OpenCloud service
// config (and thus the whole OpenCloud/reva dependency tree). The original
// binds via github.com/gookit/config; this reimplementation uses
// gopkg.in/yaml.v3 to keep the adapter kit's dependency surface slim.
//
// Behavioural note: unlike the gookit-based original, this loader does NOT
// expand ${ENV} references embedded inside yaml values. Environment overrides
// are meant to be applied separately via `env` struct tags (e.g. OpenCloud's
// standard-library-only envdecode package), after this file is loaded.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// BindSourcesToStructs loads the yaml config file for the named service into
// dst, using dst's `yaml` struct tags. The file is looked up at
// BaseConfigPath()/<service>.yaml. A missing file is not an error: adapter
// services are expected to run env-only by default, with the yaml file as an
// optional override source.
//
// dst must be a non-nil pointer to a struct.
func BindSourcesToStructs(service string, dst any) error {
	path := filepath.Join(BaseConfigPath(), service+".yaml")

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("adapter-kit: reading config file %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("adapter-kit: parsing config file %q: %w", path, err)
	}
	return nil
}

// BaseConfigPath returns the directory that holds service yaml config files.
// It honours the shared OpenCloud OC_CONFIG_DIR variable and falls back to the
// conventional /etc/opencloud, so adapter config sits alongside OpenCloud's.
func BaseConfigPath() string {
	if p := os.Getenv("OC_CONFIG_DIR"); p != "" {
		return p
	}
	return "/etc/opencloud"
}
