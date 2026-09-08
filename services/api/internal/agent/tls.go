package agent

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

var ErrInvalidTLSConfiguration = errors.New("invalid agent TLS configuration")

type TLSFiles struct {
	CertificateFile string
	PrivateKeyFile  string
	CAFile          string
	ServerName      string
}

func ClientTLSConfig(files TLSFiles) (*tls.Config, error) {
	certificate, roots, err := loadTLSMaterial(files)
	if err != nil {
		return nil, err
	}
	if files.ServerName == "" {
		return nil, ErrInvalidTLSConfiguration
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate},
		RootCAs:      roots,
		ServerName:   files.ServerName,
		// Do not set InsecureSkipVerify; server identity must be verified.
	}, nil
}

func ServerTLSConfig(files TLSFiles) (*tls.Config, error) {
	certificate, roots, err := loadTLSMaterial(files)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    roots,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}, nil
}

func loadTLSMaterial(files TLSFiles) (tls.Certificate, *x509.CertPool, error) {
	if files.CertificateFile == "" || files.PrivateKeyFile == "" || files.CAFile == "" {
		return tls.Certificate{}, nil, ErrInvalidTLSConfiguration
	}
	certificate, err := tls.LoadX509KeyPair(files.CertificateFile, files.PrivateKeyFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("load agent certificate: %w", err)
	}
	caBytes, err := os.ReadFile(files.CAFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read agent CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caBytes) {
		return tls.Certificate{}, nil, ErrInvalidTLSConfiguration
	}
	return certificate, roots, nil
}
