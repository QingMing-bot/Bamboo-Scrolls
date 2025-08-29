package secret

import (
	"encoding/base64"
	"errors"
	"runtime"
	"strings"
)

// Prefix 标识已加密字段。
const Prefix = "enc:"

// EncryptString 加密（Windows 上使用 DPAPI；其它平台直接返回原文以保持兼容）。
func EncryptString(s string) (string, error) {
	if s == "" {
		return s, nil
	}
	if strings.HasPrefix(s, Prefix) {
		return s, nil
	}
	if runtime.GOOS != "windows" {
		// 非 Windows 暂不实际加密，直接返回原文（可后续扩展跨平台方案）。
		return s, nil
	}
	b, err := dpapiProtect([]byte(s))
	if err != nil {
		return "", err
	}
	return Prefix + base64.StdEncoding.EncodeToString(b), nil
}

// DecryptString 解密；若不是加密格式则原样返回以兼容旧数据。
func DecryptString(s string) (string, error) {
	if s == "" {
		return s, nil
	}
	if !strings.HasPrefix(s, Prefix) {
		return s, nil
	}
	enc := strings.TrimPrefix(s, Prefix)
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		// 其它平台无法解 Windows DPAPI，加提示。
		return "", errors.New("encrypted key cannot be decrypted on non-windows platform")
	}
	plain, err := dpapiUnprotect(raw)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// Windows 具体实现与非 Windows stub 位于对应 build tag 文件中 (secret_windows.go / secret_stub.go)。
