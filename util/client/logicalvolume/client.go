package logicalvolume

import (
	"context"

	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/util/conversion"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PatchFn func(obj client.Object) (client.Patch, error)

func List(ctx context.Context, c client.Reader, lvlist *topolvmv1.LogicalVolumeList, opts ...client.ListOption) error {
	if topolvm.UseLegacy() {
		legacylvlist := new(topolvmlegacyv1.LogicalVolumeList)
		if err := c.List(ctx, legacylvlist, opts...); err != nil {
			return err
		}
		*lvlist = *conversion.ConvertListToCurrent(legacylvlist)
	} else {
		if err := c.List(ctx, lvlist, opts...); err != nil {
			return err
		}
	}
	return nil
}

func Get(ctx context.Context, c client.Reader, key client.ObjectKey, lv *topolvmv1.LogicalVolume) error {
	if topolvm.UseLegacy() {
		legacylv := new(topolvmlegacyv1.LogicalVolume)
		if err := c.Get(ctx, key, legacylv); err != nil {
			return err
		}
		*lv = *conversion.ConvertToCurrent(legacylv)
	} else {
		if err := c.Get(ctx, key, lv); err != nil {
			return err
		}
	}
	return nil
}

func Create(ctx context.Context, c client.Writer, lv *topolvmv1.LogicalVolume, opts ...client.CreateOption) error {
	if topolvm.UseLegacy() {
		legacylv := conversion.ConvertToLegacy(lv)
		if err := c.Create(ctx, legacylv, opts...); err != nil {
			return err
		}
		*lv = *conversion.ConvertToCurrent(legacylv)
	} else {
		if err := c.Create(ctx, lv, opts...); err != nil {
			return err
		}
	}
	return nil
}

func Update(ctx context.Context, c client.Writer, lv *topolvmv1.LogicalVolume, opts ...client.UpdateOption) error {
	if topolvm.UseLegacy() {
		legacylv := conversion.ConvertToLegacy(lv)
		if err := c.Update(ctx, legacylv, opts...); err != nil {
			return err
		}
		*lv = *conversion.ConvertToCurrent(legacylv)
	} else {
		if err := c.Update(ctx, lv, opts...); err != nil {
			return err
		}
	}
	return nil
}

func Patch(ctx context.Context, c client.Writer, lv, patchObj *topolvmv1.LogicalVolume, patchfn PatchFn, opts ...client.PatchOption) error {
	if topolvm.UseLegacy() {
		legacylv := conversion.ConvertToLegacy(lv)
		patch, err := patchfn(patchObj)
		if err != nil {
			return err
		}
		if err := c.Patch(ctx, legacylv, patch); err != nil {
			return err
		}
		*lv = *conversion.ConvertToCurrent(legacylv)
	} else {
		patch, err := patchfn(patchObj)
		if err != nil {
			return err
		}
		if err := c.Patch(ctx, lv, patch); err != nil {
			return err
		}
	}
	return nil
}

func Delete(ctx context.Context, c client.Writer, lv *topolvmv1.LogicalVolume, opts ...client.DeleteOption) error {
	if topolvm.UseLegacy() {
		legacylv := conversion.ConvertToLegacy(lv)
		if err := c.Delete(ctx, legacylv, opts...); err != nil {
			return err
		}
	} else {
		if err := c.Delete(ctx, lv, opts...); err != nil {
			return err
		}
	}
	return nil
}

func StatusUpdate(ctx context.Context, c client.StatusClient, lv *topolvmv1.LogicalVolume, opts ...client.UpdateOption) error {
	if topolvm.UseLegacy() {
		legacylv := conversion.ConvertToLegacy(lv)
		if err := c.Status().Update(ctx, legacylv, opts...); err != nil {
			return err
		}
		*lv = *conversion.ConvertToCurrent(legacylv)
	} else {
		if err := c.Status().Update(ctx, lv, opts...); err != nil {
			return err
		}
	}
	return nil
}
