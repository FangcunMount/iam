package profilelink

import (
	"context"
	"sync"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// stubProfileLinkRepo 是包内特化的测试 stub，用于返回可控的 FindByProfileID 结果
// （同时保留 Create/Update 等方法以实现 Repository 接口）
type stubProfileLinkRepo struct {
	profilesResults map[uint64][]*ProfileLink
	userResults     map[uint64][]*ProfileLink
	createArgs      []*ProfileLink
	createErr       error
	findErr         error
	isLinkedErr     error
	linked          bool
	isLinkedCalls   int
	findByUserErr   error
	findCalls       int
}

func (s *stubProfileLinkRepo) Create(_ context.Context, link *ProfileLink) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.createArgs = append(s.createArgs, link)
	return nil
}
func (s *stubProfileLinkRepo) FindByID(context.Context, meta.ID) (*ProfileLink, error) {
	return nil, nil
}
func (s *stubProfileLinkRepo) FindByProfileID(ctx context.Context, id meta.ID) ([]*ProfileLink, error) {
	s.findCalls++
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.profilesResults == nil {
		return nil, nil
	}
	return s.profilesResults[id.Uint64()], nil
}
func (s *stubProfileLinkRepo) FindByProfileIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*ProfileLink, error) {
	return s.FindByProfileID(ctx, id)
}
func (s *stubProfileLinkRepo) FindByUserID(_ context.Context, id meta.ID) ([]*ProfileLink, error) {
	if s.findByUserErr != nil {
		return nil, s.findByUserErr
	}
	if s.userResults == nil {
		return nil, nil
	}
	return s.userResults[id.Uint64()], nil
}

func (s *stubProfileLinkRepo) FindActiveByUserIDAndType(ctx context.Context, userID meta.ID, typ Type) ([]*ProfileLink, error) {
	return s.FindByUserID(ctx, userID)
}
func (s *stubProfileLinkRepo) FindByUserIDAndTypeIncludingRevoked(ctx context.Context, userID meta.ID, typ Type) ([]*ProfileLink, error) {
	return s.FindByUserID(ctx, userID)
}
func (s *stubProfileLinkRepo) FindByUserIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*ProfileLink, error) {
	return s.FindByUserID(ctx, id)
}
func (s *stubProfileLinkRepo) FindByUserIDAndProfileID(_ context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error) {
	return s.findByUserIDAndProfileID(userID, profileID, false)
}
func (s *stubProfileLinkRepo) FindByUserIDAndProfileIDIncludingRevoked(_ context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error) {
	return s.findByUserIDAndProfileID(userID, profileID, true)
}
func (s *stubProfileLinkRepo) findByUserIDAndProfileID(userID meta.ID, profileID meta.ID, includeRevoked bool) (*ProfileLink, error) {
	s.findCalls++
	if s.findErr != nil {
		return nil, s.findErr
	}
	for _, link := range s.profilesResults[profileID.Uint64()] {
		if link == nil || link.User != userID {
			continue
		}
		if !includeRevoked && !link.IsActive() {
			continue
		}
		return link, nil
	}
	return nil, nil
}
func (s *stubProfileLinkRepo) IsLinked(context.Context, meta.ID, meta.ID) (bool, error) {
	s.isLinkedCalls++
	if s.isLinkedErr != nil {
		return false, s.isLinkedErr
	}
	return s.linked, nil
}
func (s *stubProfileLinkRepo) Update(context.Context, *ProfileLink) error { return nil }

// seqProfileLinkRepo 提供按调用序列返回不同结果的 FindByProfileID，用于并发行为测试
type seqProfileLinkRepo struct {
	mu              sync.Mutex
	calls           int
	responses       [][]*ProfileLink
	isLinkedCalls   int
	linkedResponses []bool
}

func (s *seqProfileLinkRepo) FindActiveByUserIDAndType(ctx context.Context, userID meta.ID, typ Type) ([]*ProfileLink, error) {
	return s.FindByUserID(ctx, userID)
}

func (s *seqProfileLinkRepo) FindByUserIDAndTypeIncludingRevoked(ctx context.Context, userID meta.ID, typ Type) ([]*ProfileLink, error) {
	return s.FindByUserID(ctx, userID)
}

func (s *seqProfileLinkRepo) Create(context.Context, *ProfileLink) error { return nil }
func (s *seqProfileLinkRepo) FindByID(context.Context, meta.ID) (*ProfileLink, error) {
	return nil, nil
}
func (s *seqProfileLinkRepo) FindByProfileID(ctx context.Context, id meta.ID) ([]*ProfileLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls < len(s.responses) {
		res := s.responses[s.calls]
		s.calls++
		return res, nil
	}
	return s.responses[len(s.responses)-1], nil
}
func (s *seqProfileLinkRepo) FindByProfileIDIncludingRevoked(ctx context.Context, id meta.ID) ([]*ProfileLink, error) {
	return s.FindByProfileID(ctx, id)
}
func (s *seqProfileLinkRepo) FindByUserID(context.Context, meta.ID) ([]*ProfileLink, error) {
	return nil, nil
}
func (s *seqProfileLinkRepo) FindByUserIDIncludingRevoked(context.Context, meta.ID) ([]*ProfileLink, error) {
	return nil, nil
}
func (s *seqProfileLinkRepo) FindByUserIDAndProfileID(context.Context, meta.ID, meta.ID) (*ProfileLink, error) {
	return nil, nil
}
func (s *seqProfileLinkRepo) FindByUserIDAndProfileIDIncludingRevoked(context.Context, meta.ID, meta.ID) (*ProfileLink, error) {
	return nil, nil
}
func (s *seqProfileLinkRepo) IsLinked(context.Context, meta.ID, meta.ID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isLinkedCalls < len(s.linkedResponses) {
		res := s.linkedResponses[s.isLinkedCalls]
		s.isLinkedCalls++
		return res, nil
	}
	if len(s.linkedResponses) > 0 {
		return s.linkedResponses[len(s.linkedResponses)-1], nil
	}
	return false, nil
}
func (s *seqProfileLinkRepo) Update(context.Context, *ProfileLink) error { return nil }

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
