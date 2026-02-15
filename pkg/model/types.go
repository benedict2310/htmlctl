package model

// Metadata is shared by resource documents.
type Metadata struct {
	Name string `yaml:"name"`
}

type WebsiteSpec struct {
	DefaultStyleBundle string `yaml:"defaultStyleBundle"`
	BaseTemplate       string `yaml:"baseTemplate"`
}

type Website struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   Metadata    `yaml:"metadata"`
	Spec       WebsiteSpec `yaml:"spec"`
}

type PageLayoutItem struct {
	Include string `yaml:"include" json:"include"`
}

type PageSpec struct {
	Route       string           `yaml:"route"`
	Title       string           `yaml:"title"`
	Description string           `yaml:"description"`
	Layout      []PageLayoutItem `yaml:"layout"`
}

type Page struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       PageSpec `yaml:"spec"`
}

type Component struct {
	Name  string
	Scope string
	HTML  string
}

type StyleBundle struct {
	Name       string
	TokensCSS  string
	DefaultCSS string
}

type Asset struct {
	Name string
	Path string
}

type Site struct {
	RootDir    string
	Website    Website
	Pages      map[string]Page
	Components map[string]Component
	Styles     StyleBundle
	ScriptPath string
	Assets     []Asset
}
