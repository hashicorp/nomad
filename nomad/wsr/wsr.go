package wsr

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

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
	path      string
}

func NewWSRChecker(log log.Logger, pubKeyPEMPath string) (*WSRChecker, error) {
	//PubPEM, err := os.ReadFile("/Users/juanita.delacuestamorales/go/src/github.com/hashicorp/nomad/public.pem")
	pubPEM, err := os.ReadFile(pubKeyPEMPath)
	if err != nil {
		log.Error("wsr: unable to read public key file", "error", err)
		return nil, errors.New("no file!!!!")
	}

	wsr := &WSRChecker{
		publicKey: nil,
		path:      pubKeyPEMPath,
	}

	if len(pubPEM) == 0 {
		log.Info("Empty key, WSR disabled")

		return nil, ErrNoKey
	}

	var block *pem.Block
	if block, _ = pem.Decode(pubPEM); block == nil {
		return wsr, ErrInvalidKey
	}

	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			parsedKey = cert.PublicKey
		} else {
			log.Error("unable to parse key", "error", err)
			return wsr, ErrInvalidKey
		}
	}

	var pkey *ecdsa.PublicKey
	var ok bool
	if pkey, ok = parsedKey.(*ecdsa.PublicKey); !ok {
		return wsr, ErrInvalidType
	}

	wsr.publicKey = pkey

	return wsr, nil
}

func createSignature(jobspec []byte) []byte {
	privatePEM, err := os.ReadFile("/Users/juanita.delacuestamorales/go/src/github.com/hashicorp/nomad/private.pem")
	if err != nil {
		panic("oh no!!!")
	}

	block, _ := pem.Decode(privatePEM)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		fmt.Println("No key in block")

		return []byte{}
	}

	x509EncodedPrv := block.Bytes

	prKey, err := x509.ParseECPrivateKey(x509EncodedPrv)
	if err != nil {
		fmt.Println("unable to parse key", "error", err)
		return []byte{}
	}

	hash := sha256.Sum256(jobspec)

	sig, err := ecdsa.SignASN1(rand.Reader, prKey, hash[:])
	if err != nil {
		fmt.Println("unable to generate signature", "error", err)
		return []byte{}
	}

	fmt.Printf("signature: %x\n", sig)
	return sig
}

func checkFiles(jobspec string) error {
	fileSpec, err := os.ReadFile("/Users/juanita.delacuestamorales/go/src/github.com/hashicorp/heraclitus/job.nomad.hcl")
	if err != nil {
		return errors.New("no fileSpec!!!!")
	}

	// Trim to ignore trailing newlines or spaces if needed
	actual := strings.TrimSpace(string(jobspec))
	expected := strings.TrimSpace(string(fileSpec))

	// Compare
	if actual == expected {
		fmt.Println("File content matches the expected string.")
	} else {
		fmt.Println("File content does NOT match.")
		fmt.Printf("Got: %q\nExpected: %q\n", actual, strings.TrimSpace(jobspec))
		return errors.New("bad bad bad")
	}

	minLen := len(actual)
	if len(expected) < minLen {
		minLen = len(expected)
	}

	for i := 0; i < minLen; i++ {
		if actual[i] != expected[i] {
			fmt.Printf("Difference at position %d: actual '%c' != expected '%c'\n", i, actual[i], expected[i])
		}
	}

	if len(actual) != len(expected) {
		fmt.Printf("Length mismatch: actual is %d bytes, expected is %d bytes\n", len(actual), len(expected))
		if len(actual) > len(expected) {
			fmt.Printf("Extra actual content: %q\n", actual[len(expected):])
		} else {
			fmt.Printf("Missing expected content: %q\n", expected[len(actual):])
		}
	}

	return nil
}

func (wsrc *WSRChecker) CheckJobSpec(jobSpec string, signature string) bool {
	checkFiles(jobSpec)
	//signature = createSignature([]byte(jobSpec))
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		wsrc.logger.Error("Unable to base64 decode signature")
		return false
	}

	hash := sha256.Sum256([]byte(strings.TrimSpace(jobSpec)))
	v := ecdsa.VerifyASN1(wsrc.publicKey, hash[:], decoded)

	return v
}

func (wsrc *WSRChecker) Enabled() bool {
	return wsrc.publicKey != nil
}
