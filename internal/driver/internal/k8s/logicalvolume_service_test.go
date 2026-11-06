package k8s

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// emptyReader simulates a cache that has not observed any LogicalVolume yet.
type emptyReader struct {
	client.Reader
}

func (r *emptyReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return nil
}

type listCall struct {
	fieldSelector string
	items         int
}

// recordingReader records the field selector and the number of returned items of each List call.
type recordingReader struct {
	client.Reader
	rejectFieldSelector bool
	calls               []listCall
}

func (r *recordingReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	listOpts := (&client.ListOptions{}).ApplyOptions(opts)
	selector := ""
	if listOpts.FieldSelector != nil {
		selector = listOpts.FieldSelector.String()
	}
	if selector != "" && r.rejectFieldSelector {
		r.calls = append(r.calls, listCall{fieldSelector: selector})
		return apierrors.NewBadRequest(`field label not supported: status.volumeID`)
	}
	if err := r.Reader.List(ctx, list, opts...); err != nil {
		return err
	}
	r.calls = append(r.calls, listCall{
		fieldSelector: selector,
		items:         len(list.(*topolvmv1.LogicalVolumeList).Items),
	})
	return nil
}

func createLVWithVolumeID(ctx context.Context, name, volumeID string) {
	lv := &topolvmv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:     name,
			NodeName: "node",
			Size:     *resource.NewQuantity(1<<30, resource.BinarySI),
		},
	}
	Expect(k8sClient.Create(ctx, lv)).To(Succeed())
	DeferCleanup(func(ctx context.Context) {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, lv))).To(Succeed())
	})
	lv.Status.VolumeID = volumeID
	Expect(k8sClient.Status().Update(ctx, lv)).To(Succeed())
}

var _ = Describe("volumeGetter", func() {
	var prefix string

	BeforeEach(func(ctx context.Context) {
		prefix = fmt.Sprintf("vg-%d", GinkgoParallelProcess())
		for i := range 3 {
			createLVWithVolumeID(ctx, fmt.Sprintf("%s-%s-%d", prefix, "lv", i), fmt.Sprintf("%s-vol-%d", prefix, i))
		}
	})

	It("should list only the matching LogicalVolume with a field selector on cache miss", func(ctx context.Context) {
		apiReader := &recordingReader{Reader: k8sClient}
		getter := &volumeGetter{cacheReader: &emptyReader{}, apiReader: apiReader}

		lv, err := getter.Get(ctx, prefix+"-vol-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(lv.Name).To(Equal(prefix + "-lv-1"))
		Expect(apiReader.calls).To(Equal([]listCall{
			{fieldSelector: indexFieldVolumeID + "=" + prefix + "-vol-1", items: 1},
		}))
	})

	It("should return ErrVolumeNotFound without listing all LogicalVolumes", func(ctx context.Context) {
		apiReader := &recordingReader{Reader: k8sClient}
		getter := &volumeGetter{cacheReader: &emptyReader{}, apiReader: apiReader}

		_, err := getter.Get(ctx, prefix+"-vol-missing")
		Expect(err).To(MatchError(ErrVolumeNotFound))
		Expect(apiReader.calls).To(Equal([]listCall{
			{fieldSelector: indexFieldVolumeID + "=" + prefix + "-vol-missing", items: 0},
		}))
	})

	It("should fall back to listing all LogicalVolumes if the field selector is not supported", func(ctx context.Context) {
		apiReader := &recordingReader{Reader: k8sClient, rejectFieldSelector: true}
		getter := &volumeGetter{cacheReader: &emptyReader{}, apiReader: apiReader}

		lv, err := getter.Get(ctx, prefix+"-vol-2")
		Expect(err).NotTo(HaveOccurred())
		Expect(lv.Name).To(Equal(prefix + "-lv-2"))
		Expect(apiReader.calls).To(HaveLen(2))
		Expect(apiReader.calls[0].fieldSelector).To(Equal(indexFieldVolumeID + "=" + prefix + "-vol-2"))
		Expect(apiReader.calls[1].fieldSelector).To(BeEmpty())
		Expect(apiReader.calls[1].items).To(BeNumerically(">=", 3))

		_, err = getter.Get(ctx, prefix+"-vol-missing")
		Expect(err).To(MatchError(ErrVolumeNotFound))
	})
})
