// Package config provides configuration-loading helpers shared by LibreGraph
// adapter services: an OpenCloud-style yaml config-file loader.
//
// BindSourcesToStructs and BaseConfigPath are copied, with minimal
// adjustments, from opencloud-eu/opencloud (Apache-2.0): pkg/config/helpers.go
// and pkg/config/defaults/paths.go. They are reproduced here so adapter
// services can load an OpenCloud-style yaml config file WITHOUT depending on
// the opencloud module at all (importing opencloud/pkg/config drags in every
// OpenCloud service config and thus the whole OpenCloud/reva tree; even
// importing only opencloud/pkg/config/defaults would pull the opencloud module
// into go.mod and let its dependency pins, e.g. libre-graph-api-go, override
// the kit's). Behaviour and dependencies (gookit) match the original.
package config

import (
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	gofig "github.com/gookit/config/v2"
	gooyaml "github.com/gookit/config/v2/yaml"
)

// decoderConfigTagName sets the tag name to be used from the config structs.
// Currently we only support "yaml" because we only support config loading from
// yaml files and the yaml parser has no simple way to set a custom tag name.
var decoderConfigTagName = "yaml"

// BindSourcesToStructs assigns any config value from a config file / env
// variable to struct `dst`.
func BindSourcesToStructs(service string, dst interface{}) error {
	fileSystem := os.DirFS("/")
	filePath := strings.TrimLeft(path.Join(BaseConfigPath(), service+".yaml"), "/")
	return bindSourcesToStructs(fileSystem, filePath, service, dst)
}

func bindSourcesToStructs(fileSystem fs.FS, filePath, service string, dst interface{}) error {
	cnf := gofig.NewWithOptions(service)
	cnf.WithOptions(func(options *gofig.Options) {
		options.ParseEnv = true
		options.DecoderConfig.TagName = decoderConfigTagName
	})
	cnf.AddDriver(gooyaml.Driver)

	yamlContent, err := fs.ReadFile(fileSystem, filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	_ = cnf.LoadSources("yaml", yamlContent)

	err = cnf.BindStruct("", &dst)
	if err != nil {
		return err
	}

	return nil
}

var (
	// BaseConfigPathType switches between modes: "homedir" or "path".
	BaseConfigPathType = "homedir"
	// BaseConfigPathValue is the default config path in "path" mode.
	BaseConfigPathValue = "/etc/opencloud"
)

// BaseConfigPath returns the directory that holds service yaml config files,
// honouring OC_CONFIG_DIR and otherwise following the OpenCloud convention.
func BaseConfigPath() string {
	p := os.Getenv("OC_CONFIG_DIR")
	if p != "" {
		return p
	}

	switch BaseConfigPathType {
	case "homedir":
		dir, err := os.UserHomeDir()
		if err != nil {
			// fallback to BaseConfigPathValue for users without home
			return BaseConfigPathValue
		}
		return path.Join(dir, ".opencloud", "config")
	case "path":
		return BaseConfigPathValue
	default:
		log.Fatalf("BaseConfigPathType %s not found", BaseConfigPathType)
		return ""
	}
}
