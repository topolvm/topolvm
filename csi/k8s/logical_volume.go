package k8s

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm/csi"
	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LogicalVolumeService struct {
	k8sClient client.Client
	k8sCache  cache.Cache
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

	cacheClient, err := cache.New(config, cache.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	return &LogicalVolumeService{
		k8sClient: k8sClient,
		k8sCache:  cacheClient,
	}, nil
}

func (s *LogicalVolumeService) CreateVolume(ctx context.Context, node string, name string, size int64) (string, error) {
	log.Info("k8s.CreateVolume", map[string]interface{}{
		"name": name,
		"node": node,
		"size": size,
	})

	wg := &sync.WaitGroup{}
	wg.Add(1)

	lv := &topolvmv1.LogicalVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LogicalVolume",
			APIVersion: "topolvm.cybozu.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:     name,
			NodeName: node,
			Size:     *resource.NewQuantity(size, resource.BinarySI),
		},
	}

	err := s.k8sClient.Create(ctx, lv)
	if err != nil {
		return "", err
	}
	log.Info("Created!!", nil)

	//TODO: use informer
	for i := 0; i < 10; i++ {
		var newLV topolvmv1.LogicalVolume
		err := s.k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, &newLV)
		if err != nil {
			return "", err
		}
		if newLV.Status.Phase == "CREATED" && newLV.Status.VolumeID != "" {
			return newLV.Status.VolumeID, nil
		}
		if newLV.Status.Message != "" {
			return "", errors.New(newLV.Status.Message)
		}
		time.Sleep(1 * time.Second)
	}

	return "", errors.New("timed out")
}

func (s *LogicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	panic("implement me")
}

func (s *LogicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, size int64) error {
	panic("implement me")
}
