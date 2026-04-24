package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

type Service struct {
	caCert *x509.Certificate
	caKey  *rsa.PrivateKey
}

func NewService(caCertPath, caKeyPath string) (*Service, error) {
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read ca cert: %w", err)
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read ca key: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return nil, fmt.Errorf("decode ca cert pem")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse ca cert: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, fmt.Errorf("decode ca key pem")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse ca key: %w", err)
	}

	return &Service{
		caCert: caCert,
		caKey:  caKey,
	}, nil
}

type DeviceBundle struct {
	CertificatePEM string
	PrivateKeyPEM  string
}

func (s *Service) GenerateDeviceCertificate(deviceID string) (*DeviceBundle, error) {
	// 1. Generate unique private key for the device
	deviceKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate device key: %w", err)
	}

	// 2. Prepare the certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: deviceID,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	// 3. Sign the device certificate with our Root CA
	deviceCertBytes, err := x509.CreateCertificate(rand.Reader, &template, s.caCert, &deviceKey.PublicKey, s.caKey)
	if err != nil {
		return nil, fmt.Errorf("create device cert: %w", err)
	}

	// 4. Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: deviceCertBytes,
	})

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(deviceKey),
	})

	return &DeviceBundle{
		CertificatePEM: string(certPEM),
		PrivateKeyPEM:  string(keyPEM),
	}, nil
}
