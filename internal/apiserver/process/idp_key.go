package process

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
)

// parseIDPEncryptionKey 解析 IDP 加密密钥，支持 base64、base64url、hex 或纯 32 字节字符串。
func parseIDPEncryptionKey(rawSecret string) ([]byte, bool, error) {
	secret := strings.TrimSpace(rawSecret)
	if secret == "" {
		return nil, false, nil
	}

	type decoder struct {
		name   string
		decode func(string) ([]byte, error)
	}

	decoders := []decoder{
		{name: "base64", decode: base64.StdEncoding.DecodeString},
		{name: "base64_raw", decode: base64.RawStdEncoding.DecodeString},
		{name: "base64_url", decode: base64.URLEncoding.DecodeString},
		{name: "base64_url_raw", decode: base64.RawURLEncoding.DecodeString},
		{name: "hex", decode: hex.DecodeString},
	}

	for _, d := range decoders {
		if decoded, err := d.decode(secret); err == nil {
			if len(decoded) == 32 {
				return decoded, true, nil
			}
			log.Warnf("IDP encryption key decoded via %s but length was %d bytes, expected 32", d.name, len(decoded))
		}
	}

	if len(secret) == 32 {
		return []byte(secret), true, nil
	}

	return nil, true, fmt.Errorf("invalid encryption key: unable to decode to 32 bytes")
}
