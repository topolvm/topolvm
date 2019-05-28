package k8s

import (
	"context"
	"errors"
	"sync"

	"github.com/cybozu-go/topolvm/csi"
	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LogicalVolumeService struct {
	k8sClient   client.Client
	k8sInformer cache.Informer
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
	informer, err := cacheClient.GetInformerForKind(schema.GroupVersionKind{
		Group:   "topolvm.cybozu.com",
		Version: "v1",
		Kind:    "LogicalVolume",
	})

	return &LogicalVolumeService{
		k8sClient:   k8sClient,
		k8sInformer: informer,
	}, nil
}

func (s *LogicalVolumeService) CreateVolume(ctx context.Context, node string, name string, size int64) (string, error) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	lv := &topolvmv1.LogicalVolume{
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:     name,
			NodeName: node,
			Size:     *resource.NewQuantity(size, resource.BinarySI),
		},
	}

	err := s.k8sClient.Create(ctx, lv)
	if err != nil {
		return "", nil
	}

	var volumeID string
	message := "timed out"
	s.k8sInformer.AddEventHandler(&toolscache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			newLV := newObj.(*topolvmv1.LogicalVolume)
			if newLV.Name != lv.Name || newLV.Spec.NodeName != lv.Spec.NodeName {
				return
			}
			if newLV.Status.Phase != "CREATED" {
				return
			}
			if len(newLV.Status.Message) != 0 {
				message = newLV.Status.Message
			} else {
				volumeID = newLV.Status.VolumeID
			}
			wg.Done()
		},
	})

	wg.Wait()

	if len(message) != 0 {
		return "", errors.New(message)
	}
	return volumeID, nil
}

func (s *LogicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	panic("implement me")
}

func (s *LogicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, size int64) error {
	panic("implement me")
}
