package service

import (
	"errors"
	"strings"
)

// Administrator explicitly opted for plaintext configuration and display.
func publicTokenGuardConfig(c AccountTokenGuardConfig) AccountTokenGuardConfig { return c }

const tokenGuardEncryptedPrefix = "encrypted:v1:"

// SetEncryptor must be called during construction before the service is used.
func (s *AccountTokenGuardService) SetEncryptor(e SecretEncryptor) { s.encryptor = e }
func (s *AccountTokenGuardService) encodeConfig(raw string) (string, error) {
	return raw, nil
}

func (s *AccountTokenGuardService) decodeConfig(raw string) (string, error) {
	if !strings.HasPrefix(raw, tokenGuardEncryptedPrefix) {
		return raw, nil
	}
	if s.encryptor == nil {
		return "", errors.New("凭证守护配置缺少解密器")
	}
	decoded, err := s.encryptor.Decrypt(strings.TrimPrefix(raw, tokenGuardEncryptedPrefix))
	if err != nil {
		return "", errors.New("凭证守护配置解密失败")
	}
	return decoded, nil
}
