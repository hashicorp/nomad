package wsr

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"

	log "github.com/hashicorp/go-hclog"
)

const (
	TypeSensitive  = "sensitive"
	TypeHardened   = "hardened"
	TypeUnhardened = "unhardened"
)

var ErrInvalidKey = errors.New("Provided key is invalid")
var ErrInvalidType = errors.New("Not ECDSA public key")
var ErrNoKey = errors.New("Private key is mandatory")
var ErrInvalidPEM = errors.New("Failed to decode PEM block containing public key")

type validator interface {
}

type WSRChecker struct {
	publicKey *ecdsa.PublicKey
	logger    log.Logger
}

func NewWSRChecker(log log.Logger, pubKeyPEMPath string) (*WSRChecker, error) {
	//PubPEM, err := os.ReadFile("/Users/juanita.delacuestamorales/go/src/github.com/hashicorp/nomad/public.pem")
	PubPEM, err := os.ReadFile(pubKeyPEMPath)
	if err != nil {
		log.Error("wsr: unable to read public key file", "error", err)
		return nil, errors.New("no file!!!!")
	}

	wsr := &WSRChecker{
		publicKey: nil,
	}

	if len(PubPEM) == 0 {
		log.Info("Empty key, WSR disabled")

		return nil, ErrNoKey
	}

	block, _ := pem.Decode(PubPEM)
	if block == nil || block.Type != "PUBLIC KEY" {
		return wsr, ErrInvalidKey
	}
	x509EncodedPub := block.Bytes

	pubKeyInterface, err := x509.ParsePKIXPublicKey(x509EncodedPub)
	if err != nil {
		log.Error("unable to parse key", "error", err)
		return wsr, ErrInvalidKey
	}

	pubKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return wsr, ErrInvalidType
	}
	wsr.publicKey = pubKey

	return wsr, nil
}

func (wsrc *WSRChecker) CheckJobSpec(jobSpec string, signature []byte) bool {
	hash := sha256.Sum256([]byte(jobSpec))
	return ecdsa.VerifyASN1(wsrc.publicKey, hash[:], signature)
}

func (wsrc *WSRChecker) Enabled() bool {
	return wsrc.publicKey == nil
}
