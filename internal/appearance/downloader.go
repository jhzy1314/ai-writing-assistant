package appearance

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const downloadTimeout = 30 * time.Second

func (r *Resource) NeedsDownload() bool {
	_, err := os.Stat(r.LocalPath())
	return err != nil
}

func (r *Resource) Download() error {
	if r.DestDir != "" {
		if err := os.MkdirAll(r.DestDir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(r.URL)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回 %d", resp.StatusCode)
	}

	tmpPath := r.LocalPath() + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}

	h := md5.New()
	written, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("写入失败(已写入 %d 字节): %w", written, err)
	}
	f.Close()

	if r.MD5 != "" {
		got := fmt.Sprintf("%x", h.Sum(nil))
		if got != r.MD5 {
			os.Remove(tmpPath)
			return fmt.Errorf("MD5 校验失败: 期望 %s, 实际 %s", r.MD5, got)
		}
	}

	if err := os.Rename(tmpPath, r.LocalPath()); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名失败: %w", err)
	}

	return nil
}

func (r *Resource) Exists() bool {
	_, err := os.Stat(r.LocalPath())
	return err == nil
}

type resourceEntry struct {
	ID   string `yaml:"id"`
	URL  string `yaml:"url"`
	File string `yaml:"file"`
	MD5  string `yaml:"md5"`
}

type appearanceConfig struct {
	Resources []resourceEntry `yaml:"resources"`
}

func PresetsFromConfigYAML(dataDir, configPath string) ([]Resource, error) {
	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var cfg struct {
		Appearance appearanceConfig `yaml:"appearance"`
	}
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("解析外观配置失败: %w", err)
	}

	var resources []Resource
	base := filepath.Join(dataDir, "covers")
	for _, re := range cfg.Appearance.Resources {
		resources = append(resources, Resource{
			ID:       re.ID,
			URL:      re.URL,
			FileName: re.File,
			MD5:      re.MD5,
			DestDir:  base,
		})
	}
	return resources, nil
}
