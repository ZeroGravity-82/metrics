// Пакет encryption реализует гибридное шифрование метрик: payload шифруется через AES-256-GCM, а ключ сессии -
// через RSA.
//
// RSA не подходит для шифрования произвольного payload целиком: асимметричный алгоритм работает только с небольшими
// блоками данных, размер которых ограничен длиной ключа и схемой padding. Поэтому для payload динамически
// генерируется случайный одноразовый AES-ключ, которым шифруются сами данные.
//
// После этого AES-ключ шифруется RSA-публичным ключом и передается вместе с ciphertext. Такая схема сочетает
// эффективность симметричного шифрования для payload с безопасной передачей ключа через асимметричное шифрование.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// Encrypt шифрует данные по гибридной схеме: данные - AES-256-GCM, ключ сессии - RSA-OAEP.
// В encryptedData сохраняется nonce, за которым идет ciphertext вместе с GCM-тегом.
func Encrypt(data []byte, publicKeyPath string) (encryptedData, encryptedKey []byte, err error) {
	const aes256KeySize = 32

	publicKey, err := readPublicKey(publicKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read public key: %w", err)
	}

	aesKey, err := generateRandom(aes256KeySize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate session key: %w", err)
	}

	aesBlock, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce, err := generateRandom(aesGCM.NonceSize())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nil, nonce, data, nil)
	encryptedData = make([]byte, 0, len(nonce)+len(ciphertext))
	encryptedData = append(encryptedData, nonce...)
	encryptedData = append(encryptedData, ciphertext...)

	encryptedKey, err = rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, aesKey, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt session key: %w", err)
	}

	return encryptedData, encryptedKey, nil
}

// readPublicKey загружает публичный ключ из указанного файла.
func readPublicKey(path string) (*rsa.PublicKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing public key: %s", path)
	}
	switch block.Type {
	case "PUBLIC KEY":
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKIX public key: %w", err)
		}
		publicKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not RSA: %T", key)
		}
		return publicKey, nil
	case "RSA PUBLIC KEY":
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS1 public key: %w", err)
		}
		return key, nil
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("certificate public key is not RSA: %T", publicKey)
		}
		return publicKey, nil
	default:
		return nil, fmt.Errorf("unsupported public key PEM type %q", block.Type)
	}
}

// generateRandom генерирует криптостойкие случайные байты.
func generateRandom(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Decrypt расшифровывает ключ сессии через RSA-OAEP, затем извлекает nonce и расшифровывает payload через AES-256-GCM.
func Decrypt(encryptedData, encryptedKey []byte, privateKeyPath string) (plaintext []byte, err error) {
	privateKey, err := readPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt session key: %w", err)
	}

	aesBlock, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("encrypted data is too short")
	}

	nonce := encryptedData[:nonceSize]
	ciphertext := encryptedData[nonceSize:]

	plaintext, err = aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt payload: %w", err)
	}

	return plaintext, nil
}

// readPrivateKey загружает RSA-приватный ключ из PKCS#1 или PKCS#8 PEM-файла.
func readPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode private PEM block containing private key: %s", path)
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS1 private key: %w", err)
		}
		return key, nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		privateKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA: %T", key)
		}
		return privateKey, nil
	default:
		return nil, fmt.Errorf("unsupported private key PEM type %q", block.Type)
	}
}
