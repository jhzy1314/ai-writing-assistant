package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	once       sync.Once
	logDir     string
	currentDay string
	logFile    *os.File
	mu         sync.Mutex
)

// Init 初始化文件日志，每天轮转，同时保留 stdout 输出
func Init(dir string) error {
	var err error
	once.Do(func() {
		logDir = dir
		if e := os.MkdirAll(dir, 0755); e != nil {
			err = e
			return
		}
		err = rotate()
		if err != nil {
			return
		}
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		// 启动定时轮转检测
		go watchRotate()
	})
	return err
}

func rotate() error {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
	}
	today := time.Now().Format("2006-01-02")
	currentDay = today
	f, err := os.OpenFile(filepath.Join(logDir, today+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("log: open %s: %w", today+".log", err)
	}
	logFile = f
	return nil
}

func watchRotate() {
	for {
		now := time.Now()
		// 计算到明天 00:00 的时间
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		time.Sleep(tomorrow.Sub(now) + time.Second)
		rotate()
		// 清理 30 天前的日志
		cleanOld(30)
	}
}

func cleanOld(days int) {
	cutoff := time.Now().AddDate(0, 0, -days)
	entries, _ := os.ReadDir(logDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".log" {
			continue
		}
		fullPath := filepath.Join(logDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(fullPath)
		}
	}
}

// Close 关闭日志文件
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}
