package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"io/ioutil"
	"os"
)

const (
	nonceLen        = 12
	symmetricKeyLen = 32
)

var sharedKeyFile = "shared.key"

func GenerateSharedKey(filename string) error {
	sharedKeyFile = filename

	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		return nil
	}

	key, err := newSecureRandom(symmetricKeyLen)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(sharedKeyFile, key, 0400)
}

func SetSharedKeyFile(file string) {
	sharedKeyFile = file
}

func fetchSharedKey() ([]byte, error) {
	return ioutil.ReadFile(sharedKeyFile)
}

func newSecureRandom(size int) (random []byte, err error) {
	random = make([]byte, size)
	_, err = io.ReadFull(rand.Reader, random)
	return
}

// Encrypt encrypts a message ([]byte) with AES GCM and prepends the nonce to the associated []byte
func Encrypt(msg []byte) (encMsg []byte, err error) {
	key, err := fetchSharedKey()
	if err != nil {
		return
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	nonce, err := newSecureRandom(nonceLen)
	if err != nil {
		return
	}

	encMsg = aesgcm.Seal(nil, nonce, msg, nil)
	encMsg = append(nonce, encMsg...)
	return
}

// EncryptToString encrypts a message, prepends the nonce, and base64 encodes the resulting []byte
func EncryptToString(msg []byte) (encMsg string, err error) {
	data, err := Encrypt(msg)
	if err != nil {
		return
	}
	encMsg = base64.StdEncoding.EncodeToString(data)
	return
}

// Decrypt decrypts a message ([]byte) encrypted with AES GCM
func Decrypt(encMsg []byte) (msg []byte, err error) {
	key, err := fetchSharedKey()
	if err != nil {
		return
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	nonce := encMsg[:nonceLen]
	msg, err = aesgcm.Open(nil, nonce, encMsg[nonceLen:], nil)
	return
}

// DecryptFromString decodes a string from base64 to a []byte and then decrypts the result with AES GCM
func DecryptFromString(encMsg string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encMsg)
	if err != nil {
		return nil, err
	}
	return Decrypt(data)
}
