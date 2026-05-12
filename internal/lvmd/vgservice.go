package lvmd

import (
	"context"
	"errors"
	"sync"

	"github.com/topolvm/topolvm/internal/lvmd/command"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
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

	// mu protects watcherCounter and watchers. must take it when use them.
	mu             sync.Mutex
	watcherCounter int
	watchers       map[int]chan struct{}
}

func (s *vgService) GetLVList(ctx context.Context, req *proto.GetLVListRequest) (*proto.GetLVListResponse, error) {
	dc, err := s.dcManager.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	pool, err := storagePoolForDeviceClass(ctx, dc)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get pool from device class: %v", err)
	}

	lvs, err := pool.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	vols := make([]*proto.LogicalVolume, 0, len(lvs))
	for _, lv := range lvs {
		if dc.Type == lvmdTypes.TypeThick && lv.IsThin() {
			// do not send thin lvs if request is on TypeThick
			continue
		}

		vols = append(vols, &proto.LogicalVolume{
			Name:      lv.Name(),
			SizeBytes: int64(lv.Size()),
			DevMajor:  lv.MajorNumber(),
			DevMinor:  lv.MinorNumber(),
			Tags:      lv.Tags(),
			Path:      lv.Path(),
			Attr:      lv.Attr(),
		})
	}
	return &proto.GetLVListResponse{Volumes: vols}, nil
}

func (s *vgService) GetFreeBytes(ctx context.Context, req *proto.GetFreeBytesRequest) (*proto.GetFreeBytesResponse, error) {
	dc, err := s.dcManager.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	pool, err := storagePoolForDeviceClass(ctx, dc)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get pool from device class: %v", err)
	}
	free, err := pool.Free(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get free bytes: %v", err)
	}

	spare := GetSpare(dc)
	if free < spare {
		free = 0
	} else {
		free -= spare
	}

	return &proto.GetFreeBytesResponse{
		FreeBytes: free,
	}, nil
}

func (s *vgService) send(server proto.VGService_WatchServer) error {
	vgs, err := command.ListVolumeGroups(server.Context())
	if err != nil {
		return err
	}
	res := &proto.WatchResponse{}
	for _, vg := range vgs {
		vgFree, err := vg.Free()
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		vgSize, err := vg.Size()
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		pools, err := vg.ListPools(server.Context(), "")
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		for _, pool := range pools {
			dc, err := s.dcManager.FindDeviceClassByThinPoolName(vg.Name(), pool.Name())
			// we either get nil or ErrDeviceClassNotFound
			if errors.Is(err, ErrDeviceClassNotFound) {
				continue
			}

			// if we find a device class then it'll be a thin target
			tpi := &proto.ThinPoolItem{}
			pool, err := vg.FindPool(server.Context(), dc.ThinPoolConfig.Name)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			tpu, err := pool.Usage(server.Context())
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}

			// used for updating prometheus metrics
			tpi.DataPercent = tpu.DataPercent
			tpi.MetadataPercent = tpu.MetadataPercent

			var opb uint64
			if dc.ThinPoolConfig.SkipOverprovisioningRatio {
				opb = tpu.FreePoolBytes()
			} else {
				// used for annotating the node for capacity aware scheduling
				opb, err = tpu.FreeOverprovisionedBytes(dc.ThinPoolConfig.OverprovisionRatio)
				if err != nil {
					return status.Errorf(codes.Internal, "failed to get pool usage: %v", err)
				}
			}
			tpi.OverprovisionBytes = opb
			if dc.Default {
				res.FreeBytes = opb
			}

			// size bytes of the thinpool
			tpi.SizeBytes = tpu.SizeBytes

			// include thinpoolitem in the response
			res.Items = append(res.Items, &proto.WatchItem{
				DeviceClass: dc.Name,
				FreeBytes:   vgFree,
				SizeBytes:   vgSize,
				ThinPool:    tpi,
			})
		}

		dc, err := s.dcManager.FindDeviceClassByVGName(vg.Name())
		if errors.Is(err, ErrDeviceClassNotFound) {
			continue
		}

		spare := GetSpare(dc)
		if vgFree < spare {
			vgFree = 0
		} else {
			vgFree -= spare
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

	// Initial notification on startup
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
