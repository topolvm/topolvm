package lvmd

import (
	"context"
	"sync"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewVGService creates a VGServiceServer
func NewVGService(vg *command.VolumeGroup, spareGB uint64) (proto.VGServiceServer, func()) {
	svc := &vgService{
		vg:       vg,
		spare:    spareGB << 30,
		watchers: make(map[int]chan struct{}),
	}

	return svc, svc.notifyWatchers
}

type vgService struct {
	vg    *command.VolumeGroup
	spare uint64

	mu             sync.Mutex
	watcherCounter int
	watchers       map[int]chan struct{}
}

func (s *vgService) GetLVList(context.Context, *proto.Empty) (*proto.GetLVListResponse, error) {
	lvs, err := s.vg.ListVolumes()
	if err != nil {
		log.Error("failed to list volumes", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	vols := make([]*proto.LogicalVolume, len(lvs))
	for i, lv := range lvs {
		vols[i] = &proto.LogicalVolume{
			Name:     lv.Name(),
			SizeGb:   (lv.Size() + (1 << 30) - 1) >> 30,
			DevMajor: lv.MajorNumber(),
			DevMinor: lv.MinorNumber(),
		}
	}
	return &proto.GetLVListResponse{Volumes: vols}, nil
}

func (s *vgService) GetFreeBytes(context.Context, *proto.Empty) (*proto.GetFreeBytesResponse, error) {
	vgFree, err := s.vg.Free()
	if err != nil {
		log.Error("failed to free VG", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	if vgFree < s.spare {
		vgFree = 0
	} else {
		vgFree -= s.spare
	}

	return &proto.GetFreeBytesResponse{
		FreeBytes: vgFree,
	}, nil
}

func (s *vgService) send(server proto.VGService_WatchServer) error {
	vgFree, err := s.vg.Free()
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return server.Send(&proto.WatchResponse{
		FreeBytes: vgFree,
	})
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
