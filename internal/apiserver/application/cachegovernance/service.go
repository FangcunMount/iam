package cachegovernance

import (
	"context"
	"fmt"
	"sort"
	"strings"

	cacheinfra "github.com/FangcunMount/iam/internal/apiserver/infra/cache"
)

// Option 用于扩展只读治理服务的构造参数。
type Option func(*ReadService)

// WithRuntimeReaders 配置后端级运行状态读取器。
func WithRuntimeReaders(readers ...cacheinfra.RuntimeStatusReader) Option {
	return func(s *ReadService) {
		for _, reader := range readers {
			if reader == nil {
				continue
			}
			s.runtimeReaders[reader.Backend()] = reader
		}
	}
}

// ReadService 负责聚合 IAM 缓存目录和运行状态。
type ReadService struct {
	inspectors     map[cacheinfra.Family]cacheinfra.FamilyInspector
	runtimeReaders map[cacheinfra.BackendKind]cacheinfra.RuntimeStatusReader
}

// NewReadService 创建只读治理聚合服务。
func NewReadService(inspectors []cacheinfra.FamilyInspector, opts ...Option) *ReadService {
	service := &ReadService{
		inspectors:     make(map[cacheinfra.Family]cacheinfra.FamilyInspector, len(inspectors)),
		runtimeReaders: map[cacheinfra.BackendKind]cacheinfra.RuntimeStatusReader{},
	}
	for _, inspector := range inspectors {
		if inspector == nil {
			continue
		}
		service.inspectors[inspector.Descriptor().Family] = inspector
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

// Catalog 返回当前缓存目录快照。
func (s *ReadService) Catalog(context.Context) ([]cacheinfra.FamilyDescriptor, error) {
	return cacheinfra.Families(), nil
}

// Family 返回指定缓存族的静态描述和只读状态。
func (s *ReadService) Family(ctx context.Context, family cacheinfra.Family) (FamilyView, error) {
	descriptor, ok := cacheinfra.GetFamily(family)
	if !ok {
		return FamilyView{}, fmt.Errorf("unknown cache family %q", family)
	}
	return s.readFamilyView(ctx, descriptor), nil
}

// Overview 返回所有缓存族和后端运行状态的聚合视图。
func (s *ReadService) Overview(ctx context.Context) (Overview, error) {
	descriptors := cacheinfra.Families()
	views := make([]FamilyView, 0, len(descriptors))
	grouped := map[cacheinfra.BackendKind][]FamilyView{}

	for _, descriptor := range descriptors {
		view := s.readFamilyView(ctx, descriptor)
		views = append(views, view)
		grouped[descriptor.Backend] = append(grouped[descriptor.Backend], view)
	}

	runtimeStatuses := make([]cacheinfra.RuntimeStatus, 0, len(grouped))
	for _, backend := range orderedBackends(descriptors) {
		runtimeStatuses = append(runtimeStatuses, s.readRuntimeStatus(ctx, backend, grouped[backend]))
	}

	return Overview{
		RuntimeStatuses: runtimeStatuses,
		Families:        views,
	}, nil
}

func (s *ReadService) readFamilyView(ctx context.Context, descriptor cacheinfra.FamilyDescriptor) FamilyView {
	inspector := s.inspectors[descriptor.Family]
	if inspector == nil {
		return FamilyView{
			Descriptor: descriptor,
			Status: cacheinfra.FamilyStatus{
				Family:          descriptor.Family,
				Configured:      false,
				Healthy:         false,
				EntryCountKnown: false,
				Notes:           []string{"未注册 FamilyInspector，当前只返回静态目录信息。"},
			},
		}
	}

	status, err := inspector.Status(ctx)
	if err != nil {
		status = cacheinfra.FamilyStatus{
			Family:          descriptor.Family,
			Configured:      true,
			Healthy:         false,
			EntryCountKnown: false,
			Notes:           []string{fmt.Sprintf("读取缓存族状态失败: %v", err)},
		}
	}
	if status.Family == "" {
		status.Family = descriptor.Family
	}

	return FamilyView{
		Descriptor: descriptor,
		Status:     status,
	}
}

func (s *ReadService) readRuntimeStatus(ctx context.Context, backend cacheinfra.BackendKind, views []FamilyView) cacheinfra.RuntimeStatus {
	if reader := s.runtimeReaders[backend]; reader != nil {
		status, err := reader.Status(ctx)
		if err == nil {
			return status
		}
		return cacheinfra.RuntimeStatus{
			Backend:    backend,
			Configured: true,
			Healthy:    false,
			Notes:      []string{fmt.Sprintf("读取后端运行状态失败: %v", err)},
		}
	}
	return deriveRuntimeStatus(backend, views)
}

func deriveRuntimeStatus(backend cacheinfra.BackendKind, views []FamilyView) cacheinfra.RuntimeStatus {
	status := cacheinfra.RuntimeStatus{
		Backend: backend,
		Notes:   []string{},
	}
	if len(views) == 0 {
		status.Notes = append(status.Notes, "当前后端没有关联缓存族。")
		return status
	}

	configuredFamilies := make([]string, 0, len(views))
	unconfiguredFamilies := make([]string, 0, len(views))
	unhealthyFamilies := make([]string, 0, len(views))
	for _, view := range views {
		if view.Status.Configured {
			configuredFamilies = append(configuredFamilies, string(view.Descriptor.Family))
		} else {
			unconfiguredFamilies = append(unconfiguredFamilies, string(view.Descriptor.Family))
		}
		if !view.Status.Healthy {
			unhealthyFamilies = append(unhealthyFamilies, string(view.Descriptor.Family))
		}
	}

	status.Configured = len(configuredFamilies) > 0
	status.Healthy = status.Configured && len(unconfiguredFamilies) == 0 && len(unhealthyFamilies) == 0
	status.Notes = append(status.Notes, fmt.Sprintf("关联缓存族数量: %d", len(views)))
	if len(unconfiguredFamilies) > 0 {
		sort.Strings(unconfiguredFamilies)
		status.Notes = append(status.Notes, "未配置缓存族: "+strings.Join(unconfiguredFamilies, ", "))
	}
	if len(unhealthyFamilies) > 0 {
		sort.Strings(unhealthyFamilies)
		status.Notes = append(status.Notes, "不健康缓存族: "+strings.Join(unhealthyFamilies, ", "))
	}
	if status.Healthy {
		status.Notes = append(status.Notes, "关联缓存族状态正常。")
	}
	return status
}

func orderedBackends(descriptors []cacheinfra.FamilyDescriptor) []cacheinfra.BackendKind {
	seen := map[cacheinfra.BackendKind]struct{}{}
	backends := make([]cacheinfra.BackendKind, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if _, ok := seen[descriptor.Backend]; ok {
			continue
		}
		seen[descriptor.Backend] = struct{}{}
		backends = append(backends, descriptor.Backend)
	}
	return backends
}
