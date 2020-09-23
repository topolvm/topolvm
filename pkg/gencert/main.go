package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	host     = flag.String("host", "controller.topolvm-system.svc", "TLS hostname")
	validFor = flag.Duration("duration", 36500*24*time.Hour, "Duration that certificate is valid for")
	outDir   = flag.String("outdir", ".", "Directory where the certificate files are created")
)

func main() {
	flag.Parse()

	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		log.Fatal(err)
	}

	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	notBefore := time.Now()
	notAfter := notBefore.Add(*validFor)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "controller",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsAliases(*host),
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		log.Fatalf("failed to create certificate: %v", err)
	}

	_, err = os.Stat(*outDir)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		err = os.MkdirAll(*outDir, 0755)
		if err != nil {
			log.Fatalf("failed to create output directory: %v", err)
		}
	default:
		log.Fatalf("stat %s failed: %v", *outDir, err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("failed to marshal private key: %v", err)
	}

	outputPEM(filepath.Join(*outDir, "cert.pem"), "CERTIFICATE", certBytes)
	outputPEM(filepath.Join(*outDir, "key.pem"), "PRIVATE KEY", privBytes)
}

func dnsAliases(host string) []string {
	parts := strings.Split(host, ".")
	aliases := make([]string, len(parts))
	for i := 0; i < len(parts); i++ {
		aliases[i] = strings.Join(parts[0:len(parts)-i], ".")
	}
	return aliases
}

func outputPEM(fname string, pemType string, data []byte) {
	f, err := os.OpenFile(fname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("failed to open %s: %v", fname, err)
	}
	defer f.Close()

	err = pem.Encode(f, &pem.Block{Type: pemType, Bytes: data})
	if err != nil {
		log.Fatalf("failed to encode: %v", err)
	}

	err = f.Sync()
	if err != nil {
		log.Fatalf("failed to fsync: %v", err)
	}
}
