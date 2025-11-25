package executor

import (
	"context"
	"fmt"

	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type CleanerExecutor struct {
	client        client.Client
	operation     topolvmv1.OperationType
	logicalVolume *topolvmv1.LogicalVolume
}

func NewCleanerExecutor(client client.Client, logicalVolume *topolvmv1.LogicalVolume, operation topolvmv1.OperationType) *CleanerExecutor {
	return &CleanerExecutor{
		client:        client,
		operation:     operation,
		logicalVolume: logicalVolume,
	}
}

func (e *CleanerExecutor) Execute() error {
	ctx := context.Background()
	logger := log.FromContext(ctx)
	objMeta := buildObjectMeta(e.operation, e.logicalVolume)
	logger.Info("Cleaning up", "objMeta", objMeta)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objMeta.Name,
			Namespace: objMeta.Namespace,
		},
	}
	if err := e.client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err == nil {
		logger.Info("Deleting snapshot pod", "pod", pod.Name)
		fmt.Println("######################## Deleting Snapshot Pod ########################")
		fmt.Println("###################@#@## Pod Name:", pod.Name)
		if err := e.client.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete snapshot pod: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get snapshot pod: %w", err)
	}
	logger.Info("Cleanup completed successfully")
	return nil
}
