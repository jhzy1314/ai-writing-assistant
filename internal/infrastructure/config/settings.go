package config

import (
	"encoding/json"
	"os"
	"sync"
)

type LLMSettings struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

type AppSettings struct {
	Writing LLMSettings `json:"writing"`
	Review  LLMSettings `json:"review"`
}

var (
	settings     *AppSettings
	settingsPath string
	settingsMu   sync.RWMutex
)

func InitSettings(path string, defaultWriting, defaultReview LLMSettings) error {
	settingsPath = path
	settings = &AppSettings{
		Writing: defaultWriting,
		Review:  defaultReview,
	}

	data, err := os.ReadFile(path)
	if err == nil {
		var saved AppSettings
		if json.Unmarshal(data, &saved) == nil {
			if saved.Writing.APIKey != "" {
				settings.Writing = saved.Writing
			}
			if saved.Review.APIKey != "" {
				settings.Review = saved.Review
			}
		}
	}

	return nil
}

func GetSettings() *AppSettings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	if settings == nil {
		return &AppSettings{}
	}
	return &AppSettings{
		Writing: settings.Writing,
		Review:  settings.Review,
	}
}

func UpdateSettings(s AppSettings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	if s.Writing.APIKey != "" {
		settings.Writing = s.Writing
	}
	if s.Review.APIKey != "" {
		settings.Review = s.Review
	}

	if settingsPath == "" {
		return nil
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, data, 0644)
}
