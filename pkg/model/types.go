package model

// Metadata is shared by resource documents.
type Metadata struct {
	Name string `yaml:"name"`
}

type WebsiteSpec struct {
	DefaultStyleBundle string       `yaml:"defaultStyleBundle"`
	BaseTemplate       string       `yaml:"baseTemplate"`
	Head               *WebsiteHead `yaml:"head,omitempty" json:"head,omitempty"`
	SEO                *WebsiteSEO  `yaml:"seo,omitempty" json:"seo,omitempty"`
}

type WebsiteIcons struct {
	SVG        string `yaml:"svg,omitempty" json:"svg,omitempty"`
	ICO        string `yaml:"ico,omitempty" json:"ico,omitempty"`
	AppleTouch string `yaml:"appleTouch,omitempty" json:"appleTouch,omitempty"`
}

type WebsiteHead struct {
	Icons *WebsiteIcons `yaml:"icons,omitempty" json:"icons,omitempty"`
}

type WebsiteSEO struct {
	PublicBaseURL string         `yaml:"publicBaseURL" json:"publicBaseURL,omitempty"`
	Robots        *WebsiteRobots `yaml:"robots,omitempty" json:"robots,omitempty"`
}

type WebsiteRobots struct {
	Enabled bool          `yaml:"enabled" json:"enabled"`
	Groups  []RobotsGroup `yaml:"groups,omitempty" json:"groups,omitempty"`
}

type RobotsGroup struct {
	UserAgents []string `yaml:"userAgents" json:"userAgents"`
	Allow      []string `yaml:"allow,omitempty" json:"allow,omitempty"`
	Disallow   []string `yaml:"disallow,omitempty" json:"disallow,omitempty"`
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

// PageHead contains optional SEO and share metadata rendered into the page head.
type PageHead struct {
	CanonicalURL string            `yaml:"canonicalURL" json:"canonicalURL,omitempty"`
	Meta         map[string]string `yaml:"meta" json:"meta,omitempty"`
	OpenGraph    *OpenGraph        `yaml:"openGraph" json:"openGraph,omitempty"`
	Twitter      *TwitterCard      `yaml:"twitter" json:"twitter,omitempty"`
	JSONLD       []JSONLDBlock     `yaml:"jsonLD" json:"jsonLD,omitempty"`
}

// OpenGraph contains supported og:* properties in fixed declaration order.
type OpenGraph struct {
	Type        string `yaml:"type" json:"type,omitempty"`
	URL         string `yaml:"url" json:"url,omitempty"`
	SiteName    string `yaml:"siteName" json:"siteName,omitempty"`
	Locale      string `yaml:"locale" json:"locale,omitempty"`
	Title       string `yaml:"title" json:"title,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
	Image       string `yaml:"image" json:"image,omitempty"`
}

// TwitterCard contains supported twitter:* properties in fixed declaration order.
type TwitterCard struct {
	Card        string `yaml:"card" json:"card,omitempty"`
	URL         string `yaml:"url" json:"url,omitempty"`
	Title       string `yaml:"title" json:"title,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
	Image       string `yaml:"image" json:"image,omitempty"`
}

// JSONLDBlock is a single JSON-LD block emitted in input order.
type JSONLDBlock struct {
	ID      string         `yaml:"id" json:"id,omitempty"`
	Payload map[string]any `yaml:"payload" json:"payload,omitempty"`
}

type PageSpec struct {
	Route       string           `yaml:"route"`
	Title       string           `yaml:"title"`
	Description string           `yaml:"description"`
	Layout      []PageLayoutItem `yaml:"layout"`
	Head        *PageHead        `yaml:"head" json:"head,omitempty"`
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

type BrandingAsset struct {
	Slot       string
	SourcePath string
}

type Site struct {
	RootDir    string
	Website    Website
	Pages      map[string]Page
	Components map[string]Component
	Styles     StyleBundle
	ScriptPath string
	Assets     []Asset
	Branding   map[string]BrandingAsset
}
