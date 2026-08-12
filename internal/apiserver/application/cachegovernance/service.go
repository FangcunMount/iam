package cachegovernance

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ReadService 负责聚合 IAM 缓存目录和运行状态。
type ReadService struct {
	inspectors map[Family]FamilyInspector
}

// NewReadService 创建只读治理聚合服务。
func NewReadService(inspectors []FamilyInspector) *ReadService {
	service := &ReadService{
		inspectors: make(map[Family]FamilyInspector, len(inspectors)),
	}
	for _, inspector := range inspectors {
		if inspector == nil {
			continue
		}
		service.inspectors[inspector.Descriptor().Family] = inspector
	}
	return service
}

// Catalog 返回当前缓存目录快照。
func (s *ReadService) Catalog(context.Context) ([]FamilyDescriptor, error) {
	return Families(), nil
}

// Family 返回指定缓存族的静态描述和只读状态。
func (s *ReadService) Family(ctx context.Context, family Family) (FamilyView, error) {
	descriptor, ok := GetFamily(family)
	if !ok {
		return FamilyView{}, fmt.Errorf("unknown cache family %q", family)
	}
	return s.readFamilyView(ctx, descriptor), nil
}

// Overview 返回所有缓存族和后端运行状态的聚合视图。
func (s *ReadService) Overview(ctx context.Context) (Overview, error) {
	descriptors := Families()
	views := make([]FamilyView, 0, len(descriptors))
	grouped := map[BackendKind][]FamilyView{}

	for _, descriptor := range descriptors {
		view := s.readFamilyView(ctx, descriptor)
		views = append(views, view)
		grouped[descriptor.Backend] = append(grouped[descriptor.Backend], view)
	}

	runtimeStatuses := make([]RuntimeStatus, 0, len(grouped))
	for _, backend := range orderedBackends(descriptors) {
		runtimeStatuses = append(runtimeStatuses, s.readRuntimeStatus(backend, grouped[backend]))
	}

	return Overview{
		RuntimeStatuses: runtimeStatuses,
		Families:        views,
	}, nil
}

func (s *ReadService) readFamilyView(ctx context.Context, descriptor FamilyDescriptor) FamilyView {
	inspector := s.inspectors[descriptor.Family]
	if inspector == nil {
		return FamilyView{
			Descriptor: descriptor,
			Status: FamilyStatus{
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
		status = FamilyStatus{
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

func (s *ReadService) readRuntimeStatus(backend BackendKind, views []FamilyView) RuntimeStatus {
	return deriveRuntimeStatus(backend, views)
}

func deriveRuntimeStatus(backend BackendKind, views []FamilyView) RuntimeStatus {
	status := RuntimeStatus{
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

func orderedBackends(descriptors []FamilyDescriptor) []BackendKind {
	seen := map[BackendKind]struct{}{}
	backends := make([]BackendKind, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if _, ok := seen[descriptor.Backend]; ok {
			continue
		}
		seen[descriptor.Backend] = struct{}{}
		backends = append(backends, descriptor.Backend)
	}
	return backends
}
