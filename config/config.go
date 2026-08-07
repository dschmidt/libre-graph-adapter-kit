// Package config provides configuration-loading helpers shared by LibreGraph
// adapter services: an OpenCloud-style yaml config-file loader.
//
// BindSourcesToStructs is copied, with minimal adjustments, from
// opencloud-eu/opencloud (Apache-2.0), pkg/config/helpers.go. It is reproduced
// here so adapter services can load an OpenCloud-style yaml config file WITHOUT
// importing opencloud/pkg/config, which imports every OpenCloud service config
// (and thus the whole OpenCloud/reva dependency tree). Only the light,
// stdlib-only opencloud/pkg/config/defaults package (for BaseConfigPath) is
// pulled in here. Behaviour and dependencies (gookit) match the original.
package config

import (
	"io/fs"
	"os"
	"path"
	"strings"

	gofig "github.com/gookit/config/v2"
	gooyaml "github.com/gookit/config/v2/yaml"
	"github.com/opencloud-eu/opencloud/pkg/config/defaults"
)

// decoderConfigTagName sets the tag name to be used from the config structs.
// Currently we only support "yaml" because we only support config loading from
// yaml files and the yaml parser has no simple way to set a custom tag name.
var decoderConfigTagName = "yaml"

// BindSourcesToStructs assigns any config value from a config file / env
// variable to struct `dst`.
func BindSourcesToStructs(service string, dst interface{}) error {
	fileSystem := os.DirFS("/")
	filePath := strings.TrimLeft(path.Join(defaults.BaseConfigPath(), service+".yaml"), "/")
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
