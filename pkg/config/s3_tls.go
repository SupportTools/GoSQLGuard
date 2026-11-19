package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
)

// EffectiveInsecureSkipVerify reports whether TLS certificate verification
// for S3 connections has been explicitly disabled. It is false unless
// insecureSkipVerify (S3_INSECURE_SKIP_VERIFY) or its deprecated alias
// skipCertValidation (S3_SKIP_CERT_VALIDATION) is set to true.
func (c S3Config) EffectiveInsecureSkipVerify() bool {
	return c.InsecureSkipVerify || c.SkipCertValidation
}

// EffectiveCABundlePath returns the PEM CA bundle used to verify the S3
// endpoint: caBundlePath (S3_CA_BUNDLE_PATH), falling back to the deprecated
// alias customCAPath (S3_CUSTOM_CA_PATH). Empty means system roots only.
func (c S3Config) EffectiveCABundlePath() string {
	if c.CABundlePath != "" {
		return c.CABundlePath
	}
	return c.CustomCAPath
}

// TLSConfig builds the TLS client configuration for S3 connections.
//
// Certificates are verified by default against the system roots. A CA bundle
// adds trusted roots on top of the system pool. Verification is skipped only
// when insecureSkipVerify (or the deprecated skipCertValidation) is set to
// true explicitly; in that case the CA bundle is not used.
func (c S3Config) TLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

	if c.EffectiveInsecureSkipVerify() {
		// Explicit operator opt-in via insecureSkipVerify, default false.
		tlsConfig.InsecureSkipVerify = true // #nosec G402 -- explicit config opt-out, verification is the default
		log.Printf("WARNING: TLS certificate verification is DISABLED for S3 connections (insecureSkipVerify=true)")
		return tlsConfig, nil
	}

	if path := c.EffectiveCABundlePath(); path != "" {
		rootCAs, err := x509.SystemCertPool()
		if err != nil || rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		pem, err := os.ReadFile(path) // #nosec G304 -- operator-supplied CA bundle path
		if err != nil {
			return nil, fmt.Errorf("failed to read S3 CA bundle %s: %w", path, err)
		}
		if ok := rootCAs.AppendCertsFromPEM(pem); !ok {
			return nil, fmt.Errorf("S3 CA bundle %s contains no PEM certificates", path)
		}
		tlsConfig.RootCAs = rootCAs
		log.Printf("Using S3 CA bundle from %s", path)
	}

	return tlsConfig, nil
}
