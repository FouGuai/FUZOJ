package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	OutputDir string                    `yaml:"outputDir"`
	Auth      AuthProfile               `yaml:"auth"`
	Services  map[string]ServiceProfile `yaml:"services"`
}

type AuthProfile struct {
	JWTSecret string `yaml:"jwtSecret"`
	JWTIssuer string `yaml:"jwtIssuer"`
}

type ServiceProfile struct {
	Base      string                 `yaml:"base"`
	Output    string                 `yaml:"output"`
	Overrides map[string]interface{} `yaml:"overrides"`
}

func main() {
	profilePath := flag.String("profile", "configs/dev-profile.yaml", "Path to config profile")
	outputDir := flag.String("output-dir", "", "Override output directory")
	flag.Parse()

	profilePathAbs, err := filepath.Abs(*profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve profile path failed: %v\n", err)
		os.Exit(1)
	}

	profile, err := loadProfile(profilePathAbs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load profile failed: %v\n", err)
		os.Exit(1)
	}

	if *outputDir != "" {
		profile.OutputDir = *outputDir
	}
	if profile.OutputDir == "" {
		fmt.Fprintln(os.Stderr, "output directory is required")
		os.Exit(1)
	}
	profileDir := filepath.Dir(profilePathAbs)
	if !filepath.IsAbs(profile.OutputDir) {
		profile.OutputDir = filepath.Join(profileDir, profile.OutputDir)
	}

	if err := os.MkdirAll(profile.OutputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory failed: %v\n", err)
		os.Exit(1)
	}

	serviceNames := make([]string, 0, len(profile.Services))
	for name := range profile.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	for _, name := range serviceNames {
		service := profile.Services[name]
		if service.Base == "" {
			fmt.Fprintf(os.Stderr, "service %q missing base config\n", name)
			os.Exit(1)
		}
		if !filepath.IsAbs(service.Base) {
			service.Base = filepath.Join(profileDir, service.Base)
		}

		baseConfig, err := loadYAML(service.Base)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load base config for %q failed: %v\n", name, err)
			os.Exit(1)
		}
		baseConfig = normalizeValue(baseConfig)

		if len(service.Overrides) > 0 {
			override := normalizeValue(service.Overrides)
			merged, err := mergeMap(baseConfig, override)
			if err != nil {
				fmt.Fprintf(os.Stderr, "merge overrides for %q failed: %v\n", name, err)
				os.Exit(1)
			}
			baseConfig = merged
		}
		baseConfig, err = applySharedAuth(profile, name, baseConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "apply shared auth for %q failed: %v\n", name, err)
			os.Exit(1)
		}

		outputPath, err := resolveOutputPath(profile.OutputDir, service)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve output path for %q failed: %v\n", name, err)
			os.Exit(1)
		}

		if err := writeYAML(outputPath, baseConfig); err != nil {
			fmt.Fprintf(os.Stderr, "write config for %q failed: %v\n", name, err)
			os.Exit(1)
		}
	}
}

func loadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile failed: %w", err)
	}

	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile failed: %w", err)
	}
	if len(profile.Services) == 0 {
		return nil, errors.New("profile has no services")
	}
	return &profile, nil
}

func loadYAML(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read yaml failed: %w", err)
	}

	var value interface{}
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("parse yaml failed: %w", err)
	}
	return value, nil
}

func writeYAML(path string, value interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output dir failed: %w", err)
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal yaml failed: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write yaml failed: %w", err)
	}
	return nil
}

func resolveOutputPath(outputDir string, service ServiceProfile) (string, error) {
	output := service.Output
	if output == "" {
		output = filepath.Base(service.Base)
	}
	if output == "" {
		return "", errors.New("output path is empty")
	}
	if filepath.IsAbs(output) {
		return output, nil
	}
	return filepath.Join(outputDir, output), nil
}

func normalizeValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, v := range typed {
			out[k] = normalizeValue(v)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, v := range typed {
			key, ok := k.(string)
			if !ok {
				key = fmt.Sprintf("%v", k)
			}
			out[key] = normalizeValue(v)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeValue(item))
		}
		return out
	default:
		return value
	}
}

func mergeMap(base interface{}, override interface{}) (interface{}, error) {
	baseMap, ok := base.(map[string]interface{})
	if !ok {
		return nil, errors.New("base config is not a map")
	}
	overrideMap, ok := override.(map[string]interface{})
	if !ok {
		return nil, errors.New("override config is not a map")
	}

	merged := make(map[string]interface{}, len(baseMap))
	for k, v := range baseMap {
		merged[k] = v
	}

	for key, overrideValue := range overrideMap {
		baseValue, exists := merged[key]
		if !exists {
			merged[key] = overrideValue
			continue
		}

		baseChild, baseIsMap := baseValue.(map[string]interface{})
		overrideChild, overrideIsMap := overrideValue.(map[string]interface{})
		if baseIsMap && overrideIsMap {
			combined, err := mergeMap(baseChild, overrideChild)
			if err != nil {
				return nil, err
			}
			merged[key] = combined
			continue
		}
		merged[key] = overrideValue
	}
	return merged, nil
}

func applySharedAuth(profile *Profile, serviceName string, config interface{}) (interface{}, error) {
	if profile == nil {
		return config, nil
	}
	if profile.Auth.JWTSecret == "" && profile.Auth.JWTIssuer == "" {
		return config, nil
	}
	if serviceName != "gateway" && serviceName != "user-service" {
		return config, nil
	}
	root, ok := config.(map[string]interface{})
	if !ok {
		return nil, errors.New("service config is not a map")
	}
	auth, ok := root["auth"].(map[string]interface{})
	if !ok {
		auth = map[string]interface{}{}
		root["auth"] = auth
	}
	if profile.Auth.JWTSecret != "" {
		auth["jwtSecret"] = profile.Auth.JWTSecret
	}
	if profile.Auth.JWTIssuer != "" {
		auth["jwtIssuer"] = profile.Auth.JWTIssuer
	}
	return root, nil
}
