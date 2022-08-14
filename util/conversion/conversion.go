package conversion

import (
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
)

func ConvertListToCurrent(lvlist *topolvmlegacyv1.LogicalVolumeList) *topolvmv1.LogicalVolumeList {
	curent := new(topolvmv1.LogicalVolumeList)
	for _, lv := range lvlist.Items {
		curent.Items = append(curent.Items, *ConvertToCurrent(&lv))
	}
	return curent
}

func ConvertListToLegacy(lvlist *topolvmv1.LogicalVolumeList) *topolvmlegacyv1.LogicalVolumeList {
	legacy := new(topolvmlegacyv1.LogicalVolumeList)
	for _, lv := range lvlist.Items {
		legacy.Items = append(legacy.Items, *ConvertToLegacy(&lv))
	}
	return legacy
}

func ConvertToCurrent(lv *topolvmlegacyv1.LogicalVolume) *topolvmv1.LogicalVolume {
	current := &topolvmv1.LogicalVolume{
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:        lv.Spec.Name,
			NodeName:    lv.Spec.NodeName,
			Size:        lv.Spec.Size.DeepCopy(),
			DeviceClass: lv.Spec.DeviceClass,
			Source:      lv.Spec.Source,
			AccessType:  lv.Spec.AccessType,
		},
		Status: topolvmv1.LogicalVolumeStatus{
			VolumeID: lv.Status.VolumeID,
			Code:     lv.Status.Code,
			Message:  lv.Status.Message,
		},
	}
	lv.ObjectMeta.DeepCopyInto(&current.ObjectMeta)
	if lv.Status.CurrentSize != nil {
		in, out := &lv.Status.CurrentSize, &current.Status.CurrentSize
		x := (*in).DeepCopy()
		*out = &x
	}
	return current
}

func ConvertToLegacy(lv *topolvmv1.LogicalVolume) *topolvmlegacyv1.LogicalVolume {
	legacy := &topolvmlegacyv1.LogicalVolume{
		Spec: topolvmlegacyv1.LogicalVolumeSpec{
			Name:        lv.Spec.Name,
			NodeName:    lv.Spec.NodeName,
			Size:        lv.Spec.Size.DeepCopy(),
			DeviceClass: lv.Spec.DeviceClass,
			Source:      lv.Spec.Source,
			AccessType:  lv.Spec.AccessType,
		},
		Status: topolvmlegacyv1.LogicalVolumeStatus{
			VolumeID: lv.Status.VolumeID,
			Code:     lv.Status.Code,
			Message:  lv.Status.Message,
		},
	}
	lv.ObjectMeta.DeepCopyInto(&legacy.ObjectMeta)
	if lv.Status.CurrentSize != nil {
		in, out := &lv.Status.CurrentSize, &legacy.Status.CurrentSize
		x := (*in).DeepCopy()
		*out = &x
	}
	return legacy
}
