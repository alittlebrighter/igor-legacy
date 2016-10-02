package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io"
	"io/ioutil"
	"math/big"
	"os"
)

var pubKeyCurve = elliptic.P256()

const privateKeyFile = "device.key"

func GenerateKeyPair() error {
	privateKey, err := ecdsa.GenerateKey(pubKeyCurve, rand.Reader)
	if err != nil {
		return err
	}

	privData, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return err
	}

	os.Remove(privateKeyFile)
	return ioutil.WriteFile(privateKeyFile, privData, 0400)
}

func fetchKeyPair() (*ecdsa.PrivateKey, error) {
	data, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, err
	}

	return x509.ParseECPrivateKey(data)
}

func PublicKey() (crypto.PublicKey, error) {
	privKey, err := fetchKeyPair()
	if err != nil {
		return ecdsa.PublicKey{}, err
	}
	return privKey.PublicKey, nil
}

func computeHash(msg string) (hash []byte, err error) {
	h := md5.New()
	_, err = io.WriteString(h, msg)
	if err != nil {
		return
	}
	hash = h.Sum(nil)
	return
}

func SignToString(msg string) (sig string, err error) {
	hash, err := computeHash(msg)
	privKey, err := fetchKeyPair()
	if err != nil {
		return
	}

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash)
	sig = base64.StdEncoding.EncodeToString(append(r.Bytes(), s.Bytes()...))
	return
}

func VerifyFromString(pubKey *ecdsa.PublicKey, msg, sig string) (verified bool, err error) {
	hash, err := computeHash(msg)
	decodedSig, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return
	}
	if len(decodedSig) != 64 {
		err = errors.New("Signature data not long enough.")
		return
	}

	r, s := decodedSig[:32], decodedSig[32:]

	verified = ecdsa.Verify(pubKey, hash, new(big.Int).SetBytes(r), new(big.Int).SetBytes(s))

	return
}
