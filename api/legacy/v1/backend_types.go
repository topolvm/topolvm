package v1

type S3Spec struct {
	// Endpoint specifies the URL of the S3 or S3 compatible storage bucket.
	Endpoint string `json:"endpoint,omitempty"`

	// Bucket specifies the name of the bucket that will be used as storage backend.
	Bucket string `json:"bucket,omitempty"`

	// Prefix specifies a directory inside the bucket/container where the data for this backend will be stored.
	Prefix string `json:"prefix,omitempty"`

	// Region specifies the region where the bucket is located
	// +optional
	Region string `json:"region,omitempty"`

	// SecretName specifies the name of the Secret that contains the access credential for this storage.
	// Must be in the same namespace as TopoLVM components.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// MaxConnections specifies the maximum number of concurrent connections to use to upload/download/delete data to this backend.
	// +optional
	MaxConnections int64 `json:"maxConnections,omitempty"`

	// InsecureTLS controls whether a client should skip TLS certificate verification.
	// Setting this field to true disables verification, which might be necessary in cases
	// where the server uses self-signed certificates or certificates from an untrusted CA.
	// Use this option with caution, as it can expose the client to man-in-the-middle attacks
	// and other security risks. Only use it when absolutely necessary.
	// +optional
	InsecureTLS bool `json:"insecureTLS,omitempty"`
}

type GCSSpec struct {
	// Bucket specifies the name of the bucket that will be used as storage backend.
	Bucket string `json:"bucket,omitempty"`

	// Prefix specifies a directory inside the bucket/container where the data for this backend will be stored.
	Prefix string `json:"prefix,omitempty"`

	// MaxConnections specifies the maximum number of concurrent connections to use to upload/download data to this backend.
	// +optional
	MaxConnections int64 `json:"maxConnections,omitempty"`

	// SecretName specifies the name of the Secret that contains the access credential for this storage.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

type AzureSpec struct {
	// StorageAccount specifies the name of the Azure Storage Account
	StorageAccount string `json:"storageAccount,omitempty"`

	// Container specifies the name of the Azure Blob container that will be used as storage backend.
	Container string `json:"container,omitempty"`

	// Prefix specifies a directory inside the bucket/container where the data for this backend will be stored.
	Prefix string `json:"prefix,omitempty"`

	// MaxConnections specifies the maximum number of concurrent connections to use to upload/download data to this backend.
	// +optional
	MaxConnections int64 `json:"maxConnections,omitempty"`

	// SecretName specifies the name of the Secret that contains the access credential for this storage.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}
