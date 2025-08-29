package secret

import (
	"runtime"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	plain := "test-key-material"
	enc, err := EncryptString(plain)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if runtime.GOOS == "windows" {
		if enc == plain {
			t.Fatalf("expected encrypted string with prefix, got plain")
		}
		if len(enc) <= len(Prefix) || enc[:len(Prefix)] != Prefix {
			t.Fatalf("missing prefix in %q", enc)
		}
		dec, err := DecryptString(enc)
		if err != nil {
			t.Fatalf("decrypt error: %v", err)
		}
		if dec != plain {
			t.Fatalf("decrypt mismatch got %q", dec)
		}
	} else {
		// 非 Windows 目前为直通
		if enc != plain {
			t.Fatalf("non-windows should be passthrough")
		}
		dec, err := DecryptString(enc)
		if err != nil {
			t.Fatalf("decrypt passthrough error: %v", err)
		}
		if dec != plain {
			t.Fatalf("expected passthrough decrypt got %q", dec)
		}
	}
}
