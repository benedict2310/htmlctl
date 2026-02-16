package diff

type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeModified ChangeType = "modified"
	ChangeRemoved  ChangeType = "removed"
)

type FileRecord struct {
	Path string `json:"path" yaml:"path"`
	Hash string `json:"hash" yaml:"hash"`
}

type FileChange struct {
	Path         string     `json:"path" yaml:"path"`
	ResourceType string     `json:"resourceType" yaml:"resourceType"`
	ChangeType   ChangeType `json:"changeType" yaml:"changeType"`
	OldHash      string     `json:"oldHash,omitempty" yaml:"oldHash,omitempty"`
	NewHash      string     `json:"newHash,omitempty" yaml:"newHash,omitempty"`
}

type Summary struct {
	Added     int `json:"added" yaml:"added"`
	Modified  int `json:"modified" yaml:"modified"`
	Removed   int `json:"removed" yaml:"removed"`
	Unchanged int `json:"unchanged" yaml:"unchanged"`
}

type Result struct {
	Changes []FileChange `json:"changes" yaml:"changes"`
	Summary Summary      `json:"summary" yaml:"summary"`
}

func (r Result) HasChanges() bool {
	return r.Summary.Added+r.Summary.Modified+r.Summary.Removed > 0
}
