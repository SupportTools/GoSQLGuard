package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newSelfSignedServer starts an HTTPS server with a self-signed certificate
// and writes that certificate to a PEM CA bundle file.
func newSelfSignedServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	bundle := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(bundle, pemBytes, 0o600); err != nil {
		t.Fatalf("write CA bundle: %v", err)
	}
	return srv, bundle
}

func get(t *testing.T, cfg S3Config, url string) error {
	t.Helper()
	tlsConfig, err := cfg.TLSConfig()
	if err != nil {
		t.Fatalf("TLSConfig: %v", err)
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func TestS3TLSConfig_DefaultVerifiesAndRejectsSelfSigned(t *testing.T) {
	srv, _ := newSelfSignedServer(t)

	cfg := S3Config{}
	tlsConfig, err := cfg.TLSConfig()
	if err != nil {
		t.Fatalf("TLSConfig: %v", err)
	}
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("default config must verify certificates")
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want TLS 1.2", tlsConfig.MinVersion)
	}

	err = get(t, cfg, srv.URL)
	if err == nil {
		t.Fatal("default config connected to a self-signed server; verification is not happening")
	}
	var unknownAuthority x509.UnknownAuthorityError
	if !errors.As(err, &unknownAuthority) {
		t.Fatalf("want x509.UnknownAuthorityError, got %v", err)
	}
}

func TestS3TLSConfig_CABundleTrustsServer(t *testing.T) {
	srv, bundle := newSelfSignedServer(t)

	cfg := S3Config{CABundlePath: bundle}
	if err := get(t, cfg, srv.URL); err != nil {
		t.Fatalf("CA bundle should verify the server: %v", err)
	}
	tlsConfig, _ := cfg.TLSConfig()
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("CA bundle must not disable verification")
	}
}

func TestS3TLSConfig_DeprecatedCustomCAPathStillWorks(t *testing.T) {
	srv, bundle := newSelfSignedServer(t)
	if err := get(t, S3Config{CustomCAPath: bundle}, srv.URL); err != nil {
		t.Fatalf("deprecated customCAPath should still verify the server: %v", err)
	}
}

func TestS3TLSConfig_ExplicitInsecureSkipVerify(t *testing.T) {
	srv, _ := newSelfSignedServer(t)
	if err := get(t, S3Config{InsecureSkipVerify: true}, srv.URL); err != nil {
		t.Fatalf("insecureSkipVerify=true should connect: %v", err)
	}
	if err := get(t, S3Config{SkipCertValidation: true}, srv.URL); err != nil {
		t.Fatalf("deprecated skipCertValidation=true should connect: %v", err)
	}
}

func TestS3TLSConfig_BadCABundle(t *testing.T) {
	dir := t.TempDir()
	notPEM := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(notPEM, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (S3Config{CABundlePath: notPEM}).TLSConfig(); err == nil {
		t.Fatal("expected an error for a CA bundle without PEM certificates")
	}
	if _, err := (S3Config{CABundlePath: filepath.Join(dir, "missing.pem")}).TLSConfig(); err == nil {
		t.Fatal("expected an error for a missing CA bundle")
	}
}

func TestS3TLSConfig_EnvDefaults(t *testing.T) {
	t.Setenv("S3_INSECURE_SKIP_VERIFY", "")
	t.Setenv("S3_SKIP_CERT_VALIDATION", "")
	t.Setenv("S3_CA_BUNDLE_PATH", "/etc/ssl/s3-ca.pem")
	loadFromEnvironment()
	if CFG.S3.EffectiveInsecureSkipVerify() {
		t.Fatal("insecure skip verify must default to false")
	}
	if got := CFG.S3.EffectiveCABundlePath(); got != "/etc/ssl/s3-ca.pem" {
		t.Fatalf("CA bundle path = %q", got)
	}
	t.Setenv("S3_INSECURE_SKIP_VERIFY", "true")
	loadFromEnvironment()
	if !CFG.S3.EffectiveInsecureSkipVerify() {
		t.Fatal("S3_INSECURE_SKIP_VERIFY=true must disable verification")
	}
}
