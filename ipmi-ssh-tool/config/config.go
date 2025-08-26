package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Machine 单台机器的配置信息结构
type Machine struct {
	IPMIIP   string `json:"ipmi_ip"`
	IPMIUser string `json:"ipmi_user"`
	IPMIPwd  string `json:"ipmi_pwd"`
	SSHIP    string `json:"ssh_ip"`
	SSHUser  string `json:"ssh_user"`
}

// Machines 多台机器配置列表
type Machines []Machine

// 配置文件路径(默认当前目录下的 machines.json)
const configPath = "machines.json"

// GetLocalSSHKeyPath 获取本地 SSH 公钥路径（跨平台）
func GetLocalSSHKeyPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".ssh", "id_rsa.pub")
}

// Load 从 JSON 文件加载机器配置
func Load() (Machines, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Machines{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var ms Machines
	if err := json.Unmarshal(data, &ms); err != nil {
		return nil, err
	}
	return ms, nil
}

// Save 保存机器配置到 JSON 文件
func (ms Machines) Save() error {
	data, err := json.MarshalIndent(ms, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// GetLocalSSHKey 读取本地 SSH 公钥内容
func GetLocalSSHKey() (string, error) {
	keyPath := GetLocalSSHKeyPath()
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return "", errors.New("本地 SSH 公钥不存在，请执行: ssh-keygen -t rsa")
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return "", err
	}
	return string(keyData), nil
}
