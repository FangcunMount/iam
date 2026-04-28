package mysql

import (
	"context"
	"errors"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	"github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
	dbmysql "github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"gorm.io/gorm"
)

// wechatAppRepository 微信应用仓储实现
type wechatAppRepository struct {
	dbmysql.BaseRepository[*WechatAppPO]
	dbConn *gorm.DB
}

// 确保实现了接口
var _ wechatapp.Repository = (*wechatAppRepository)(nil)

// NewWechatAppRepository 创建微信应用仓储实例
func NewWechatAppRepository(db *gorm.DB) wechatapp.Repository {
	base := dbmysql.NewBaseRepository[*WechatAppPO](db)
	base.SetErrorTranslator(dbmysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrWechatAppAlreadyExists, "wechat app already exists")
	}))

	return &wechatAppRepository{dbConn: db, BaseRepository: base}
}

// Create 创建微信应用
func (r *wechatAppRepository) Create(ctx context.Context, app *wechatapp.WechatApp) error {
	if app == nil {
		return errors.New("app cannot be nil")
	}

	po := &WechatAppPO{}
	po.FromDomain(app)

	return r.CreateAndSync(ctx, po, func(updated *WechatAppPO) {
		app.ID = updated.ID
	})
}

// GetByID 根据 ID 查询微信应用
func (r *wechatAppRepository) GetByID(ctx context.Context, id idutil.ID) (*wechatapp.WechatApp, error) {
	po, err := r.FindByID(ctx, id.Uint64())
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get wechat app by id: %w", err)
	}
	if po == nil {
		return nil, nil
	}
	return po.ToDomain(), nil
}

// List 查询微信应用列表。
func (r *wechatAppRepository) List(ctx context.Context, filter wechatapp.ListFilter) ([]*wechatapp.WechatApp, error) {
	query := r.dbConn.WithContext(ctx).Model(&WechatAppPO{})
	if filter.Type != nil {
		query = query.Where("type = ?", string(*filter.Type))
	}
	if filter.Status != nil {
		query = query.Where("status = ?", string(*filter.Status))
	}

	var pos []*WechatAppPO
	if err := query.Order("app_id ASC").Find(&pos).Error; err != nil {
		return nil, fmt.Errorf("failed to list wechat apps: %w", err)
	}

	apps := make([]*wechatapp.WechatApp, 0, len(pos))
	for _, po := range pos {
		apps = append(apps, po.ToDomain())
	}
	return apps, nil
}

// GetByAppID 根据 AppID 查询微信应用
func (r *wechatAppRepository) GetByAppID(ctx context.Context, appID string) (*wechatapp.WechatApp, error) {
	if appID == "" {
		return nil, errors.New("appID cannot be empty")
	}

	var po WechatAppPO
	if err := r.WithContext(ctx).Where("app_id = ?", appID).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get wechat app by appID: %w", err)
	}

	return po.ToDomain(), nil
}

// Update 更新微信应用
func (r *wechatAppRepository) Update(ctx context.Context, app *wechatapp.WechatApp) error {
	if app == nil {
		return errors.New("app cannot be nil")
	}

	po := &WechatAppPO{}
	po.FromDomain(app)

	// use BaseRepository helper to perform update
	result := r.dbConn.WithContext(ctx).Model(&WechatAppPO{}).Where("id = ?", po.ID).Updates(map[string]interface{}{
		"name":                   po.Name,
		"type":                   po.Type,
		"status":                 po.Status,
		"auth_secret_cipher":     po.AuthSecretCipher,
		"auth_secret_fp":         po.AuthSecretFP,
		"auth_secret_version":    po.AuthSecretVersion,
		"auth_secret_rotated_at": po.AuthSecretRotatedAt,
		"msg_callback_token":     po.MsgCallbackToken,
		"msg_aes_key_cipher":     po.MsgAESKeyCipher,
		"msg_secret_version":     po.MsgSecretVersion,
		"msg_secret_rotated_at":  po.MsgSecretRotatedAt,
	})

	if result.Error != nil {
		return fmt.Errorf("failed to update wechat app: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("wechat app not found")
	}

	return nil
}
