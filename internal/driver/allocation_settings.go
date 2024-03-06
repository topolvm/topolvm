package driver

import (
	"reflect"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/resource"
)

// MinimumAllocationSettings contains the minimum allocation settings for the controller.
// It contains the default settings.
type MinimumAllocationSettings struct {
	Filesystem map[string]Quantity `json:"filesystem" ,yaml:"filesystem"`
	Block      Quantity            `json:"block" ,yaml:"block"`
}

type Quantity resource.Quantity

var _ pflag.Value = &Quantity{}

func newQuantityValue(val resource.Quantity, p *Quantity) *Quantity {
	*p = Quantity(val)
	return p
}

func NewQuantityFlagVar(fs *pflag.FlagSet, name string, value resource.Quantity, usage string) Quantity {
	p := new(Quantity)
	fs.Var(newQuantityValue(value, p), name, usage)
	return *p
}

func QuantityVar(fs *pflag.FlagSet, p *Quantity, name string, value resource.Quantity, usage string) {
	fs.Var(newQuantityValue(value, p), name, usage)
}

func (q *Quantity) String() string {
	rq := resource.Quantity(*q)
	return rq.String()
}

func (q *Quantity) Set(s2 string) error {
	rq, err := resource.ParseQuantity(s2)
	if err != nil {
		return err
	}
	*q = Quantity(rq)
	return nil
}

func (q *Quantity) Type() string {
	rq := resource.Quantity(*q)
	return reflect.TypeOf(rq).String()
}

// MinMaxAllocationsFromSettings returns the minimum and maximum allocations based on the settings.
// It uses the required and limit bytes from the CSI Call and the device class and capabilities from the StorageClass.
// It then returns the minimum and maximum allocations in bytes that can be used for that context.
func (settings MinimumAllocationSettings) MinMaxAllocationsFromSettings(
	required, limit int64,
	capabilities []*csi.VolumeCapability,
) (int64, int64) {
	minimumSize := settings.GetMinimumAllocationSize(capabilities)

	if minimumSize.CmpInt64(required) > 0 {
		ctrlLogger.Info("required size is less than minimum size, "+
			"using minimum size as required size", "required", required, "minimum", minimumSize.Value())
		required = minimumSize.Value()
	}

	return required, limit
}

// GetMinimumAllocationSize returns the minimum size to be allocated from the parameters derived from the StorageClass.
// it uses either the filesystem or block key to get the minimum allocated size.
// it determines which key to use based on the capabilities.
// If no key is found or neither capability exists, it returns 0.
// If the value is not a valid Quantity, it returns an error.
func (settings MinimumAllocationSettings) GetMinimumAllocationSize(
	capabilities []*csi.VolumeCapability,
) resource.Quantity {
	var quantity resource.Quantity

	for _, capability := range capabilities {
		if capability.GetBlock() != nil {
			quantity = resource.Quantity(settings.Block)
			break
		}

		if capability.GetMount() != nil {
			rawQuantity, ok := settings.Filesystem[capability.GetMount().FsType]
			if !ok {
				break
			}

			quantity = resource.Quantity(rawQuantity)

			break
		}
	}

	if quantity.Sign() < 0 {
		return resource.MustParse("0")
	}

	return quantity
}
