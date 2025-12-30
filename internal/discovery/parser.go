package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// XrayConfig Xray 配置文件结构
type XrayConfig struct {
	Inbounds []struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Tag      string `json:"tag"`
		Listen   string `json:"listen"`
	} `json:"inbounds"`
}

// SingboxConfig Sing-box 配置文件结构
type SingboxConfig struct {
	Inbounds []struct {
		Type       string `json:"type"`
		Tag        string `json:"tag"`
		Listen     string `json:"listen"`
		ListenPort int    `json:"listen_port"`
	} `json:"inbounds"`
}

// ParseConfig 解析配置文件
func ParseConfig(configPath string, proxyType string) ([]Inbound, error) {
	// 读取文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 清洗注释（JSONC → JSON）
	cleaned := removeComments(string(data))

	// 根据代理类型解析
	switch proxyType {
	case "xray":
		return parseXrayConfig(cleaned)
	case "sing-box":
		return parseSingboxConfig(cleaned)
	default:
		return nil, fmt.Errorf("不支持的代理类型: %s", proxyType)
	}
}

// removeComments 移除JSON注释
func removeComments(jsonc string) string {
	// 移除单行注释: //...
	// 注意：需要避免误删URL中的 //
	re1 := regexp.MustCompile(`(?m)^\s*//.*$`)
	jsonc = re1.ReplaceAllString(jsonc, "")

	// 移除行尾注释: ... // comment
	re2 := regexp.MustCompile(`\s*//.*$`)
	jsonc = re2.ReplaceAllString(jsonc, "")

	// 移除多行注释: /* ... */
	re3 := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	jsonc = re3.ReplaceAllString(jsonc, "")

	return jsonc
}

// parseXrayConfig 解析Xray配置
func parseXrayConfig(jsonStr string) ([]Inbound, error) {
	var config XrayConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("解析Xray配置失败: %w", err)
	}

	var inbounds []Inbound
	for _, ib := range config.Inbounds {
		// 过滤本地回环监听（通常是内部通信）
		if ib.Listen == "127.0.0.1" || ib.Listen == "localhost" {
			continue
		}

		// 默认监听地址
		listen := ib.Listen
		if listen == "" {
			listen = "0.0.0.0"
		}

		inbounds = append(inbounds, Inbound{
			Port:     ib.Port,
			Protocol: ib.Protocol,
			Tag:      ib.Tag,
			Listen:   listen,
		})
	}

	return inbounds, nil
}

// parseSingboxConfig 解析Sing-box配置
func parseSingboxConfig(jsonStr string) ([]Inbound, error) {
	var config SingboxConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("解析Sing-box配置失败: %w", err)
	}

	var inbounds []Inbound
	for _, ib := range config.Inbounds {
		// 过滤本地回环监听
		if ib.Listen == "127.0.0.1" || ib.Listen == "localhost" {
			continue
		}

		// 默认监听地址
		listen := ib.Listen
		if listen == "" {
			listen = "0.0.0.0"
		}

		inbounds = append(inbounds, Inbound{
			Port:     ib.ListenPort,
			Protocol: ib.Type,
			Tag:      ib.Tag,
			Listen:   listen,
		})
	}

	return inbounds, nil
}
