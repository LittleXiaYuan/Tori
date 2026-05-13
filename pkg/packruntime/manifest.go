package packruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const ManifestFileName = "pack.json"

type Manifest struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	RequiresCore string            `json:"requiresCore,omitempty"`
	Optional     bool              `json:"optional"`
	DefaultState string            `json:"defaultState,omitempty"`
	Backend      BackendManifest   `json:"backend"`
	Frontend     FrontendManifest  `json:"frontend"`
	SDK          SDKManifest       `json:"sdk,omitempty"`
	Update       UpdateManifest    `json:"update,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type BackendManifest struct {
	Capabilities []string `json:"capabilities,omitempty"`
	Routes       []string `json:"routes,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
}

type FrontendManifest struct {
	Menus  []FrontendMenu  `json:"menus,omitempty"`
	Routes []FrontendRoute `json:"routes,omitempty"`
	Assets FrontendAssets  `json:"assets,omitempty"`
}

type FrontendMenu struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Path  string `json:"path"`
	Icon  string `json:"icon,omitempty"`
	Order int    `json:"order,omitempty"`
}

type FrontendRoute struct {
	Path      string `json:"path"`
	Component string `json:"component"`
	Title     string `json:"title,omitempty"`
}

type FrontendAssets struct {
	Type  string `json:"type,omitempty"`
	Entry string `json:"entry,omitempty"`
}

type SDKManifest struct {
	TypeScript string `json:"typescript,omitempty"`
	Go         string `json:"go,omitempty"`
	Python     string `json:"python,omitempty"`
}

type UpdateManifest struct {
	Channel  string `json:"channel,omitempty"`
	Rollback bool   `json:"rollback"`
}

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("pack manifest id is required")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("pack manifest name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("pack manifest version is required")
	}
	if m.DefaultState != "" && m.DefaultState != "enabled" && m.DefaultState != "disabled" {
		return fmt.Errorf("pack manifest defaultState must be enabled or disabled")
	}
	for i, route := range m.Backend.Routes {
		if !strings.HasPrefix(route, "/") {
			return fmt.Errorf("backend.routes[%d] must start with /", i)
		}
	}
	for i, menu := range m.Frontend.Menus {
		if strings.TrimSpace(menu.Key) == "" || strings.TrimSpace(menu.Label) == "" || strings.TrimSpace(menu.Path) == "" {
			return fmt.Errorf("frontend.menus[%d] requires key, label and path", i)
		}
	}
	for i, route := range m.Frontend.Routes {
		if strings.TrimSpace(route.Path) == "" || strings.TrimSpace(route.Component) == "" {
			return fmt.Errorf("frontend.routes[%d] requires path and component", i)
		}
	}
	return nil
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read pack manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse pack manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func SaveManifest(path string, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pack manifest: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
