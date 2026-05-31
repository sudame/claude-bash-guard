package keychain

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeHex(t *testing.T) {
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n-----END RSA PRIVATE KEY-----\n"

	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"hex-encoded PEM decodes", hex.EncodeToString([]byte(pem)), pem, true},
		{"plain PEM is left as-is", pem, "", false},
		{"odd length is not hex", "abc", "", false},
		{"empty is not hex", "", "", false},
		{"non-hex chars are not hex", "zzzz", "", false},
		{"uppercase hex decodes", "414243", "ABC", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeHex(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
