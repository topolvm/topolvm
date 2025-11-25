package config

import (
	"context"
	"fmt"

	v1 "github.com/topolvm/topolvm/api/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ofst "kmodules.xyz/offshoot-api/api/v1"
	runtime_client "sigs.k8s.io/controller-runtime/pkg/client"
)

type SetupOptions struct {
	Provider       string
	Bucket         string
	StorageAccount string
	Endpoint       string
	Region         string
	Path           string
	CacertFile     string
	ScratchDir     string
	EnableCache    bool
	InsecureTLS    bool
	MaxConnections int64
	CombineOutput  bool
	StorageSecret  *core.Secret
	Nice           *ofst.NiceSettings
	IONice         *ofst.IONiceSettings
}

func NewSetupOptionsForStorage(client runtime_client.Client, storage *v1.OnlineSnapshotStorage) (*SetupOptions, error) {
	var opt SetupOptions
	var secretName string
	if s3 := storage.Spec.Storage.S3; s3 != nil {
		opt.Provider = string(v1.ProviderS3)
		opt.Region = s3.Region
		opt.Bucket = s3.Bucket
		opt.Endpoint = s3.Endpoint
		opt.Path = s3.Prefix
		secretName = s3.SecretName
		opt.InsecureTLS = s3.InsecureTLS
		opt.MaxConnections = s3.MaxConnections
	}

	if gcs := storage.Spec.Storage.GCS; gcs != nil {
		opt.Provider = string(v1.ProviderGCS)
		opt.Bucket = gcs.Bucket
		opt.Path = gcs.Prefix
		secretName = gcs.SecretName
		opt.MaxConnections = gcs.MaxConnections
	}

	if azure := storage.Spec.Storage.Azure; azure != nil {
		opt.Provider = string(v1.ProviderAzure)
		opt.StorageAccount = azure.StorageAccount
		opt.Bucket = azure.Container
		opt.Path = azure.Prefix
		secretName = azure.SecretName
		opt.MaxConnections = azure.MaxConnections
	}

	if secretName != "" {
		secret := &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: storage.Namespace,
			},
		}
		if err := client.Get(context.Background(), runtime_client.ObjectKeyFromObject(secret), secret); err != nil {
			return nil, fmt.Errorf("failed to get secret %s: %v", secretName, err)
		}
		opt.StorageSecret = secret
	}
	return &opt, nil
}
