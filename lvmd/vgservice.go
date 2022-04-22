package lvmd

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/cybozu-go/log"
	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewVGService creates a VGServiceServer
func NewVGService(manager *DeviceClassManager) (proto.VGServiceServer, func()) {
	svc := &vgService{
		dcManager: manager,
		watchers:  make(map[int]chan struct{}),
	}

	return svc, svc.notifyWatchers
}

type vgService struct {
	proto.UnimplementedVGServiceServer
	dcManager *DeviceClassManager

	mu             sync.Mutex
	watcherCounter int
	watchers       map[int]chan struct{}
}

func (s *vgService) GetLVList(_ context.Context, req *proto.GetLVListRequest) (*proto.GetLVListResponse, error) {
	dc, err := s.dcManager.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}

	var lvs []*command.LogicalVolume

	switch dc.Type {
	case TypeThick:
		// thick logicalvolumes
		lvs, err = vg.ListVolumes()
	case TypeThin:
		var pool *command.ThinPool
		pool, err = vg.FindPool(dc.ThinPoolConfig.Name)
		if err != nil {
			return nil, err
		}
		// thin logicalvolumes
		lvs, err = pool.ListVolumes()
	default:
		// technically this block will not be hit however make sure we return error
		// in such cases where deviceclass target is neither thick or thinpool
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
	}

	if err != nil {
		log.Error("failed to list volumes", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	vols := make([]*proto.LogicalVolume, 0, len(lvs))
	for _, lv := range lvs {
		if dc.Type == TypeThick && lv.IsThin() {
			// do not send thin lvs if request is on TypeThick
			continue
		}
		vols = append(vols, &proto.LogicalVolume{
			Name:     lv.Name(),
			SizeGb:   (lv.Size() + (1 << 30) - 1) >> 30,
			DevMajor: lv.MajorNumber(),
			DevMinor: lv.MinorNumber(),
			Tags:     lv.Tags(),
		})
	}
	return &proto.GetLVListResponse{Volumes: vols}, nil
}

func (s *vgService) GetFreeBytes(_ context.Context, req *proto.GetFreeBytesRequest) (*proto.GetFreeBytesResponse, error) {
	dc, err := s.dcManager.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}

	var vgFree uint64
	switch dc.Type {
	case TypeThick:
		vgFree, err = vg.Free()
		if err != nil {
			log.Error("failed to get free bytes", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
	case TypeThin:
		pool, err := vg.FindPool(dc.ThinPoolConfig.Name)
		if err != nil {
			log.Error("failed to get thinpool", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
		tpu, err := pool.Free()
		if err != nil {
			log.Error("failed to get free bytes", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}

		// freebytes available in thinpool considering the overprovisionratio
		vgFree = uint64(math.Floor(dc.ThinPoolConfig.OverprovisionRatio*float64(tpu.SizeBytes))) - tpu.VirtualBytes

	default:
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
	}

	spare := dc.GetSpare()
	if vgFree < spare {
		vgFree = 0
	} else {
		vgFree -= spare
	}

	return &proto.GetFreeBytesResponse{
		FreeBytes: vgFree,
	}, nil
}

func (s *vgService) send(server proto.VGService_WatchServer) error {
	vgs, err := command.ListVolumeGroups()
	if err != nil {
		return err
	}
	res := &proto.WatchResponse{}
	for _, vg := range vgs {
		pools, err := vg.ListPools()
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		for _, pool := range pools {
			dc, err := s.dcManager.FindDeviceClassByThinPoolName(vg.Name(), pool.Name())
			// we either get nil or ErrNotFound
			if err == ErrNotFound {
				continue
			}

			// if we find a device class then it'll be a thin target
			tpi := &proto.ThinPoolItem{}
			pool, err := vg.FindPool(dc.ThinPoolConfig.Name)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			tpu, err := pool.Free()
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			opb := uint64(math.Floor(dc.ThinPoolConfig.OverprovisionRatio*float64(tpu.SizeBytes))) - tpu.VirtualBytes
			if dc.Default {
				// TODO (leelavg): remove this after removal of support for inline ephemeral volumes
				res.FreeBytes = opb
			}

			// used for updating prometheus metrics
			tpi.DataPercent = tpu.DataPercent
			tpi.MetadataPercent = tpu.MetadataPercent

			// used for annotating the node for capacity aware scheduling
			tpi.OverprovisionBytes = opb

			// include thinpoolitem in the response
			res.Items = append(res.Items, &proto.WatchItem{
				DeviceClass: dc.Name,
				SizeBytes:   tpu.SizeBytes,
				ThinPool:    tpi,
			})
		}

		dc, err := s.dcManager.FindDeviceClassByVGName(vg.Name())
		if err == ErrNotFound {
			continue
		}
		vgFree, err := vg.Free()
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		vgSize, err := vg.Size()
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if dc.Default {
			res.FreeBytes = vgFree
		}

		res.Items = append(res.Items, &proto.WatchItem{
			DeviceClass: dc.Name,
			FreeBytes:   vgFree,
			SizeBytes:   vgSize,
		})
	}
	return server.Send(res)
}

func (s *vgService) addWatcher(ch chan struct{}) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	num := s.watcherCounter
	s.watcherCounter++
	s.watchers[num] = ch
	return num
}

func (s *vgService) removeWatcher(num int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watchers[num]; !ok {
		panic("bug")
	}
	delete(s.watchers, num)
}

func (s *vgService) notifyWatchers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.watchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *vgService) Watch(_ *proto.Empty, server proto.VGService_WatchServer) error {
	ch := make(chan struct{}, 1)
	num := s.addWatcher(ch)
	defer s.removeWatcher(num)

	if err := s.send(server); err != nil {
		return err
	}

	for {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		case <-ch:
			if err := s.send(server); err != nil {
				return err
			}
		}
	}
}
