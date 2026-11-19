package s3

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/supporttools/GoSQLGuard/pkg/config"
)

// headBucket runs HeadBucket through the real client factory against a
// self-signed HTTPS "S3" endpoint and reports whether the request reached it.
func headBucket(t *testing.T, s3cfg config.S3Config) (reached bool, err error) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if s3cfg.CABundlePath == "ca-from-server" {
		bundle := filepath.Join(t.TempDir(), "ca.pem")
		pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
		if err := os.WriteFile(bundle, pemBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		s3cfg.CABundlePath = bundle
	}

	saved := config.CFG
	t.Cleanup(func() { config.CFG = saved })
	s3cfg.Enabled = true
	s3cfg.Endpoint = srv.URL
	s3cfg.Region = "us-east-1"
	s3cfg.Bucket = "test-bucket"
	s3cfg.AccessKey = "test"
	s3cfg.SecretKey = "test"
	s3cfg.UseSSL = true
	config.CFG.S3 = s3cfg

	client, err := getS3Client()
	if err != nil {
		t.Fatalf("getS3Client: %v", err)
	}
	_, err = client.HeadBucket(context.Background(), &awss3.HeadBucketInput{Bucket: aws.String("test-bucket")},
		func(o *awss3.Options) { o.RetryMaxAttempts = 1 })
	return hits.Load() > 0, err
}

func TestS3Client_DefaultRejectsSelfSignedEndpoint(t *testing.T) {
	reached, err := headBucket(t, config.S3Config{})
	if err == nil || reached {
		t.Fatalf("default config must fail TLS verification against a self-signed endpoint (reached=%v err=%v)", reached, err)
	}
	if !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("want a certificate verification error, got %v", err)
	}
}

func TestS3Client_CABundleTrustsEndpoint(t *testing.T) {
	reached, err := headBucket(t, config.S3Config{CABundlePath: "ca-from-server"})
	if err != nil || !reached {
		t.Fatalf("CA bundle should verify the endpoint (reached=%v err=%v)", reached, err)
	}
}

func TestS3Client_ExplicitInsecureSkipVerify(t *testing.T) {
	reached, err := headBucket(t, config.S3Config{InsecureSkipVerify: true})
	if err != nil || !reached {
		t.Fatalf("insecureSkipVerify=true should connect (reached=%v err=%v)", reached, err)
	}
}
