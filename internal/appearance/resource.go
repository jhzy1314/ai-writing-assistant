package appearance

import "path/filepath"

type Resource struct {
	ID       string
	URL      string
	FileName string
	MD5      string
	DestDir  string
}

func (r *Resource) LocalPath() string {
	return filepath.Join(r.DestDir, r.FileName)
}

type Preset struct {
	ID    string `yaml:"id"`
	URL   string `yaml:"url"`
	File  string `yaml:"file"`
	MD5   string `yaml:"md5"`
}

func DefaultResources(dataDir string) []Resource {
	base := filepath.Join(dataDir, "covers")
	return []Resource{
		{
			ID:       "empty-state",
			URL:      "https://raw.githubusercontent.com/ai-novel/studio-assets/main/backgrounds/empty-state.png",
			FileName: "empty-state.png",
			MD5:      "",
			DestDir:  base,
		},
		{
			ID:       "empty-state-light",
			URL:      "https://raw.githubusercontent.com/ai-novel/studio-assets/main/backgrounds/empty-state-light.png",
			FileName: "empty-state-light.png",
			MD5:      "",
			DestDir:  base,
		},
		{
			ID:       "sidebar-texture",
			URL:      "https://raw.githubusercontent.com/ai-novel/studio-assets/main/backgrounds/sidebar-texture.png",
			FileName: "sidebar-texture.png",
			MD5:      "",
			DestDir:  base,
		},
		{
			ID:       "sidebar-texture-light",
			URL:      "https://raw.githubusercontent.com/ai-novel/studio-assets/main/backgrounds/sidebar-texture-light.png",
			FileName: "sidebar-texture-light.png",
			MD5:      "",
			DestDir:  base,
		},
	}
}
