package k8s

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/csi"
	topolvmv1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type logicalVolumeService struct {
	k8sClient client.Client
	k8sCache  cache.Cache
	namespace string
	mu        sync.Mutex
}

// NewLogicalVolumeService returns LogicalVolumeService.
func NewLogicalVolumeService(namespace string) (csi.LogicalVolumeService, error) {
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

	err = cacheClient.IndexField(&topolvmv1.LogicalVolume{}, "status.volumeID", func(o runtime.Object) []string {
		return []string{o.(*topolvmv1.LogicalVolume).Status.VolumeID}
	})
	if err != nil {
		return nil, err
	}

	return &logicalVolumeService{
		k8sClient: k8sClient,
		k8sCache:  cacheClient,
		namespace: namespace,
	}, nil
}

func (s *logicalVolumeService) CreateVolume(ctx context.Context, node string, name string, sizeGb int64) (string, error) {
	log.Info("k8s.CreateVolume", map[string]interface{}{
		"name":    name,
		"node":    node,
		"size_gb": sizeGb,
	})

	s.mu.Lock()
	defer s.mu.Unlock()

	lv := &topolvmv1.LogicalVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LogicalVolume",
			APIVersion: "topolvm.cybozu.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.namespace,
		},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:     name,
			NodeName: node,
			Size:     *resource.NewQuantity(sizeGb<<30, resource.BinarySI),
		},
	}

	existingLV := new(topolvmv1.LogicalVolume)
	err := s.k8sClient.Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: name}, existingLV)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		err := s.k8sClient.Create(ctx, lv)
		if err != nil {
			return "", err
		}
		log.Info("Created!!", nil)
	} else {
		// LV with same name was found; check compatibility
		// skip check of capabilities because (1) we allow both of two access types, and (2) we allow only one access mode
		// for ease of comparison, sizes are compared strictly, not by compatibility of ranges
		if !existingLV.IsCompatibleWith(lv) {
			return "", status.Error(codes.AlreadyExists, "Incompatible LogicalVolume already exists")
		}
		// compatible LV was found
	}

	for {
		var newLV topolvmv1.LogicalVolume
		err := s.k8sClient.Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: name}, &newLV)
		if err != nil {
			return "", err
		}
		if newLV.Status.VolumeID != "" {
			return newLV.Status.VolumeID, nil
		}
		if newLV.Status.Code != codes.OK {
			err := s.k8sClient.Delete(ctx, &newLV)
			if err != nil {
				// log this error but do not return this error, because newLV.Status.Message is more important
				log.Error("failed to delete LogicalVolume", map[string]interface{}{
					log.FnError: err,
				})
			}
			return "", status.Error(newLV.Status.Code, newLV.Status.Message)
		}

		select {
		case <-ctx.Done():
			return "", errors.New("timed out")
		case <-time.After(1 * time.Second):
		}
	}
}

func (s *logicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	lvList := new(topolvmv1.LogicalVolumeList)
	err := s.k8sClient.List(ctx, lvList, client.InNamespace(topolvm.SystemNamespace))
	if err != nil {
		return err
	}

	lv := findByVolumeID(lvList, volumeID)
	if lv == nil {
		log.Info("volume is not found", map[string]interface{}{
			"volume_id": volumeID,
		})
		return nil
	}

	return s.k8sClient.Delete(ctx, lv)
}

func findByVolumeID(lvList *topolvmv1.LogicalVolumeList, volumeID string) *topolvmv1.LogicalVolume {
	for _, lv := range lvList.Items {
		if lv.Status.VolumeID == volumeID {
			return &lv
		}
	}
	return nil
}

func (s *logicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error {
	panic("implement me")
}
