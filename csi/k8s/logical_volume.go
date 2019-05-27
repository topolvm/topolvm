package k8s

import (
	"context"
	"github.com/cybozu-go/topolvm/csi"
	"k8s.io/client-go/rest"

	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LogicalVolumeService struct {
	k8sClient client.Client
}

func NewLogicalVolumeService() (csi.LogicalVolumeService, error) {
	err := topolvmv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	k8sClient, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	return &LogicalVolumeService{k8sClient: k8sClient}, nil
}

func (s *LogicalVolumeService) CreateVolume(ctx context.Context, node string, name string, size int64) (string, error) {
	panic("implement me")
}

func (s *LogicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	panic("implement me")
}

func (s *LogicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, size int64) error {
	panic("implement me")
}
