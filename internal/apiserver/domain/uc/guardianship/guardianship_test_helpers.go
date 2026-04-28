package guardianship

import (
	"context"
	"sync"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// stubGuardianshipRepo 是包内特化的测试 stub，用于返回可控的 FindByChildID 结果
// （同时保留 Create/Update 等方法以实现 Repository 接口）
type stubGuardianshipRepo struct {
	childrenResults map[uint64][]*Guardianship
	findErr         error
	findCalls       int
}

func (s *stubGuardianshipRepo) Create(context.Context, *Guardianship) error { return nil }
func (s *stubGuardianshipRepo) FindByID(context.Context, meta.ID) (*Guardianship, error) {
	return nil, nil
}
func (s *stubGuardianshipRepo) FindByChildID(ctx context.Context, id meta.ID) ([]*Guardianship, error) {
	s.findCalls++
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.childrenResults == nil {
		return nil, nil
	}
	return s.childrenResults[id.Uint64()], nil
}
func (s *stubGuardianshipRepo) FindByChildIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*Guardianship, error) {
	return s.FindByChildID(ctx, id)
}
func (s *stubGuardianshipRepo) FindByUserID(context.Context, meta.ID) ([]*Guardianship, error) {
	return nil, nil
}
func (s *stubGuardianshipRepo) FindByUserIDIncludingRevoked(context.Context, meta.ID) ([]*Guardianship, error) {
	return nil, nil
}
func (s *stubGuardianshipRepo) FindByUserIDAndChildID(context.Context, meta.ID, meta.ID) (*Guardianship, error) {
	return nil, nil
}
func (s *stubGuardianshipRepo) FindByUserIDAndChildIDIncludingRevoked(context.Context, meta.ID, meta.ID) (*Guardianship, error) {
	return nil, nil
}
func (s *stubGuardianshipRepo) IsGuardian(context.Context, meta.ID, meta.ID) (bool, error) {
	return false, nil
}
func (s *stubGuardianshipRepo) Update(context.Context, *Guardianship) error { return nil }

// seqGuardRepo 提供按调用序列返回不同结果的 FindByChildID，用于并发行为测试
type seqGuardRepo struct {
	mu        sync.Mutex
	calls     int
	responses [][]*Guardianship
}

func (s *seqGuardRepo) Create(context.Context, *Guardianship) error              { return nil }
func (s *seqGuardRepo) FindByID(context.Context, meta.ID) (*Guardianship, error) { return nil, nil }
func (s *seqGuardRepo) FindByChildID(ctx context.Context, id meta.ID) ([]*Guardianship, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls < len(s.responses) {
		res := s.responses[s.calls]
		s.calls++
		return res, nil
	}
	return s.responses[len(s.responses)-1], nil
}
func (s *seqGuardRepo) FindByChildIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*Guardianship, error) {
	return s.FindByChildID(ctx, id)
}
func (s *seqGuardRepo) FindByUserID(context.Context, meta.ID) ([]*Guardianship, error) {
	return nil, nil
}
func (s *seqGuardRepo) FindByUserIDIncludingRevoked(context.Context, meta.ID) ([]*Guardianship, error) {
	return nil, nil
}
func (s *seqGuardRepo) FindByUserIDAndChildID(context.Context, meta.ID, meta.ID) (*Guardianship, error) {
	return nil, nil
}
func (s *seqGuardRepo) FindByUserIDAndChildIDIncludingRevoked(context.Context, meta.ID, meta.ID) (*Guardianship, error) {
	return nil, nil
}
func (s *seqGuardRepo) IsGuardian(context.Context, meta.ID, meta.ID) (bool, error) { return false, nil }
func (s *seqGuardRepo) Update(context.Context, *Guardianship) error                { return nil }

// contains 方便在断言中检查子串
func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (indexOf(s, sub) >= 0))
}

// indexOf 是一个简单实现，避免引入 strings 包以保持测试文件与其余代码风格一致
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
