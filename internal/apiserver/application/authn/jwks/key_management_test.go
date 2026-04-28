package jwks

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
)

// MockKeyManagementService 模拟密钥管理服务
type MockKeyManagementService struct {
	mock.Mock
}

func (m *MockKeyManagementService) CreateKey(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*jwks.Key, error) {
	args := m.Called(ctx, alg, notBefore, notAfter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwks.Key), args.Error(1)
}

func (m *MockKeyManagementService) GetActiveKey(ctx context.Context) (*jwks.Key, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwks.Key), args.Error(1)
}

func (m *MockKeyManagementService) GetKeyByKid(ctx context.Context, kid string) (*jwks.Key, error) {
	args := m.Called(ctx, kid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwks.Key), args.Error(1)
}

func (m *MockKeyManagementService) RetireKey(ctx context.Context, kid string) error {
	args := m.Called(ctx, kid)
	return args.Error(0)
}

func (m *MockKeyManagementService) ForceRetireKey(ctx context.Context, kid string) error {
	args := m.Called(ctx, kid)
	return args.Error(0)
}

func (m *MockKeyManagementService) EnterGracePeriod(ctx context.Context, kid string) error {
	args := m.Called(ctx, kid)
	return args.Error(0)
}

func (m *MockKeyManagementService) CleanupExpiredKeys(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockKeyManagementService) ListKeys(ctx context.Context, status jwks.KeyStatus, limit, offset int) ([]*jwks.Key, int64, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*jwks.Key), args.Get(1).(int64), args.Error(2)
}

// createTestKey 创建测试密钥
func createTestKey(kid string, status jwks.KeyStatus, alg string) *jwks.Key {
	n := "test-modulus"
	e := "AQAB"
	publicJWK := jwks.PublicJWK{
		Kty: "RSA",
		Use: "sig",
		Alg: alg,
		Kid: kid,
		N:   &n,
		E:   &e,
	}

	now := time.Now()
	return jwks.NewKey(kid, publicJWK,
		jwks.WithStatus(status),
		jwks.WithNotBefore(now),
		jwks.WithNotAfter(now.Add(30*24*time.Hour)),
	)
}

func TestKeyManagementAppService_CreateKey(t *testing.T) {
	mockSvc := new(MockKeyManagementService)
	logger := log.New(log.NewOptions())
	appSvc := NewKeyManagementAppService(mockSvc, logger)

	ctx := context.Background()
	now := time.Now()
	testKey := createTestKey("test-kid-1", jwks.KeyActive, "RS256")

	mockSvc.On("CreateKey", ctx, "RS256", &now, mock.Anything).Return(testKey, nil)

	req := CreateKeyRequest{
		Algorithm: "RS256",
		NotBefore: &now,
	}

	resp, err := appSvc.CreateKey(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-kid-1", resp.Kid)
	assert.Equal(t, jwks.KeyActive, resp.Status)
	assert.Equal(t, "RS256", resp.Algorithm)

	mockSvc.AssertExpectations(t)
}

func TestKeyManagementAppService_GetActiveKey(t *testing.T) {
	mockSvc := new(MockKeyManagementService)
	logger := log.New(log.NewOptions())
	appSvc := NewKeyManagementAppService(mockSvc, logger)

	ctx := context.Background()
	testKey := createTestKey("active-kid", jwks.KeyActive, "RS256")

	mockSvc.On("GetActiveKey", ctx).Return(testKey, nil)

	resp, err := appSvc.GetActiveKey(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "active-kid", resp.Kid)
	assert.Equal(t, jwks.KeyActive, resp.Status)

	mockSvc.AssertExpectations(t)
}

func TestKeyManagementAppService_GetKeyByKid(t *testing.T) {
	mockSvc := new(MockKeyManagementService)
	logger := log.New(log.NewOptions())
	appSvc := NewKeyManagementAppService(mockSvc, logger)

	ctx := context.Background()
	testKey := createTestKey("test-kid", jwks.KeyGrace, "RS384")

	mockSvc.On("GetKeyByKid", ctx, "test-kid").Return(testKey, nil)

	resp, err := appSvc.GetKeyByKid(ctx, "test-kid")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-kid", resp.Kid)
	assert.Equal(t, jwks.KeyGrace, resp.Status)
	assert.Equal(t, "RS384", resp.Algorithm)

	mockSvc.AssertExpectations(t)
}

func TestKeyManagementAppService_RetireKey(t *testing.T) {
	mockSvc := new(MockKeyManagementService)
	logger := log.New(log.NewOptions())
	appSvc := NewKeyManagementAppService(mockSvc, logger)

	ctx := context.Background()

	mockSvc.On("RetireKey", ctx, "retire-kid").Return(nil)

	err := appSvc.RetireKey(ctx, "retire-kid")

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestKeyManagementAppService_ForceRetireKey(t *testing.T) {
	mockSvc := new(MockKeyManagementService)
	logger := log.New(log.NewOptions())
	appSvc := NewKeyManagementAppService(mockSvc, logger)

	ctx := context.Background()

	mockSvc.On("ForceRetireKey", ctx, "force-retire-kid").Return(nil)

	err := appSvc.ForceRetireKey(ctx, "force-retire-kid")

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestKeyManagementAppService_EnterGracePeriod(t *testing.T) {
	mockSvc := new(MockKeyManagementService)
	logger := log.New(log.NewOptions())
	appSvc := NewKeyManagementAppService(mockSvc, logger)

	ctx := context.Background()

	mockSvc.On("EnterGracePeriod", ctx, "grace-kid").Return(nil)

	err := appSvc.EnterGracePeriod(ctx, "grace-kid")

	assert.NoError(t, err)
	mockSvc.AssertExpectations(t)
}

func TestKeyManagementAppService_CleanupExpiredKeys(t *testing.T) {
	mockSvc := new(MockKeyManagementService)
	logger := log.New(log.NewOptions())
	appSvc := NewKeyManagementAppService(mockSvc, logger)

	ctx := context.Background()

	mockSvc.On("CleanupExpiredKeys", ctx).Return(5, nil)

	resp, err := appSvc.CleanupExpiredKeys(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 5, resp.DeletedCount)

	mockSvc.AssertExpectations(t)
}

func TestKeyManagementAppService_ListKeys(t *testing.T) {
	mockSvc := new(MockKeyManagementService)
	logger := log.New(log.NewOptions())
	appSvc := NewKeyManagementAppService(mockSvc, logger)

	ctx := context.Background()
	testKeys := []*jwks.Key{
		createTestKey("kid-1", jwks.KeyActive, "RS256"),
		createTestKey("kid-2", jwks.KeyGrace, "RS256"),
	}

	mockSvc.On("ListKeys", ctx, jwks.KeyStatus(0), 10, 0).Return(testKeys, int64(2), nil)

	req := ListKeysRequest{
		Limit:  10,
		Offset: 0,
	}

	resp, err := appSvc.ListKeys(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, len(resp.Keys))
	assert.Equal(t, int64(2), resp.Total)
	assert.Equal(t, "kid-1", resp.Keys[0].Kid)
	assert.Equal(t, "kid-2", resp.Keys[1].Kid)

	mockSvc.AssertExpectations(t)
}
