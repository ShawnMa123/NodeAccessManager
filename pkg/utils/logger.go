package utils

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 全局日志实例
var Logger *logrus.Logger

// InitLogger 初始化日志系统
func InitLogger(logFile string, level string, maxSize, maxBackups, maxAge int) error {
	Logger = logrus.New()

	// 设置日志级别
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	Logger.SetLevel(logLevel)

	// 设置日志格式
	Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 如果指定了日志文件，使用滚动日志
	if logFile != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(logFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// 配置滚动日志
		roller := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    maxSize,    // MB
			MaxBackups: maxBackups, // 保留备份数
			MaxAge:     maxAge,     // 天数
			Compress:   true,       // 压缩旧日志
		}

		// 同时输出到文件和终端
		multiWriter := io.MultiWriter(os.Stdout, roller)
		Logger.SetOutput(multiWriter)
	} else {
		// 仅输出到终端
		Logger.SetOutput(os.Stdout)
	}

	return nil
}

// GetLogger 获取日志实例
func GetLogger() *logrus.Logger {
	if Logger == nil {
		Logger = logrus.New()
		Logger.SetLevel(logrus.InfoLevel)
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}
	return Logger
}
