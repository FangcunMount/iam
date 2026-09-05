package attributeproviders

import (
	"fmt"
	"io"
	"os"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"gopkg.in/yaml.v3"
)

func Load(path string) (*objectattributeadmission.Registry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open authz attribute providers: %w", err)
	}
	defer func() { _ = file.Close() }()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var config struct {
		Providers []objectattributeadmission.Provider `yaml:"providers"`
	}
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode authz attribute providers: %w", err)
	}
	if config.Providers == nil {
		return nil, fmt.Errorf("attribute providers requires an explicit providers list")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("attribute providers require one YAML document")
	}
	return objectattributeadmission.New(config.Providers)
}
