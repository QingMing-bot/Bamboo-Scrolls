//go:build !windows

package secret

func dpapiProtect(b []byte) ([]byte, error)   { return b, nil }
func dpapiUnprotect(b []byte) ([]byte, error) { return b, nil }
