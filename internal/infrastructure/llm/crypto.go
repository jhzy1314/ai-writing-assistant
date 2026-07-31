package llm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// CookieEncryptor Cookie加密器
type CookieEncryptor struct {
	key []byte
}

// NewCookieEncryptor 创建加密器（使用密码派生密钥）
func NewCookieEncryptor(password string) *CookieEncryptor {
	// 使用SHA256从密码派生密钥
	hash := sha256.Sum256([]byte(password))
	return &CookieEncryptor{key: hash[:]}
}

// Encrypt 加密Cookie
func (e *CookieEncryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("创建加密器失败: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM失败: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密Cookie
func (e *CookieEncryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("解码失败: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("创建解密器失败: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM失败: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("数据格式错误")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

// 全局加密器实例（使用固定密钥，生产环境应从配置读取）
var globalEncryptor *CookieEncryptor

// InitCookieEncryptor 初始化Cookie加密器
func InitCookieEncryptor(password string) {
	globalEncryptor = NewCookieEncryptor(password)
}

// EncryptCookie 加密Cookie
func EncryptCookie(plaintext string) (string, error) {
	if globalEncryptor == nil {
		// 默认密钥（生产环境应从配置读取）
		InitCookieEncryptor("ai-novel-cookie-secret-2024")
	}
	return globalEncryptor.Encrypt(plaintext)
}

// DecryptCookie 解密Cookie
func DecryptCookie(ciphertext string) (string, error) {
	if globalEncryptor == nil {
		InitCookieEncryptor("ai-novel-cookie-secret-2024")
	}
	return globalEncryptor.Decrypt(ciphertext)
}
