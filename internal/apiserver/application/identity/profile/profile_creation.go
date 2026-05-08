package profile

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	profilDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// profileCreationInfo 档案信息
type profileCreationInfo struct {
	Name     string
	Gender   meta.Gender
	Birthday meta.Birthday
	IDCard   meta.IDCard
}

// profileCreationFields 档案创建字段
func buildProfileCreationInfo(dto CreateProfileDTO) (profileCreationInfo, error) {
	info := profileCreationInfo{
		Name:     dto.Name,
		Gender:   meta.NewGender(dto.Gender),
		Birthday: meta.NewBirthday(dto.Birthday),
	}

	idCard, hasIDCard, err := optionalIDCard(dto.Name, dto.IDCard)
	if err != nil {
		return profileCreationInfo{}, perrors.WithCode(code.ErrInvalidArgument, "无效的身份证信息")
	}
	if hasIDCard {
		info.IDCard = idCard
	}

	return info, nil
}

// optionalIDCard 根据提供的原始身份证信息尝试构建身份证对象。
// 如果原始信息为空，返回 (meta.IDCard{}, false, nil)，表示没有身份证信息。
// 如果原始信息非空但无效，返回相应的错误。
func optionalIDCard(name, raw string) (meta.IDCard, bool, error) {
	if raw == "" {
		return meta.IDCard{}, false, nil
	}
	idCard, err := meta.NewIDCard(name, raw)
	return idCard, true, err
}

// ======================================
// ==== Profile Creation 相关逻辑 =====
// ======================================

// checkProfileCreationInfo 验证创建档案所需的信息是否合法，并返回规范化后的信息。
func checkProfileCreationInfo(ctx context.Context, profiles profilDomain.Repository, info profileCreationInfo) error {
	// 验证姓名是否为空
	if info.Name == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "档案姓名不能为空")
	}

	// 验证性别是否合法（如果提供了）
	if info.Gender != 0 && !info.Gender.IsValid() {
		return perrors.WithCode(code.ErrInvalidArgument, "无效的性别值")
	}

	// 验证生日是否合法（如果提供了）
	if !info.Birthday.IsEmpty() && !info.Birthday.IsValid() {
		return perrors.WithCode(code.ErrInvalidArgument, "无效的生日值")
	}

	// 验证身份证信息（如果提供了）
	if info.IDCard.IsValid() {
		if err := profilDomain.NewIDCardUniquenessChecker(profiles).CheckIDCardUnique(ctx, info.IDCard); err != nil {
			return perrors.WithCode(code.ErrInvalidArgument, "身份证信息已存在")
		}
	}

	return nil
}

// createProfileRecord 根据规范化后的创建信息在数据库中创建档案记录，并返回创建的档案实体。
func createProfileRecord(ctx context.Context, profiles profilDomain.Repository, creationInfo profileCreationInfo) (*profilDomain.Profile, error) {
	// 构建档案实体
	newProfile, err := newProfileEntity(creationInfo)
	if err != nil {
		return nil, err
	}

	// 持久化档案实体
	if err := profiles.Create(ctx, newProfile); err != nil {
		return nil, err
	}

	return newProfile, nil
}

// newProfileEntity 根据规范化后的创建信息构建档案实体
func newProfileEntity(info profileCreationInfo) (*profilDomain.Profile, error) {
	opts := make([]profilDomain.ProfileOption, 0, 3)
	if info.Gender.IsValid() {
		opts = append(opts, profilDomain.WithGender(info.Gender))
	}
	if info.Birthday.IsValid() {
		opts = append(opts, profilDomain.WithBirthday(info.Birthday))
	}
	if info.IDCard.IsValid() {
		opts = append(opts, profilDomain.WithIDCard(info.IDCard))
	}

	return profilDomain.NewProfile(info.Name, opts...)
}

// ======================================
// ==== Profile Link 创建相关逻辑 =========
// ======================================

// createProfileLinkRecord 创建用户与档案的关系记录
func createProfileLinkRecord(
	ctx context.Context, links profileLinkDomain.Repository,
	userID meta.ID, profileID meta.ID, rel profileLinkDomain.Relation,
) (*profileLinkDomain.ProfileLink, error) {
	// 创建用户的档案关系记录
	newProfileLink, err := profileLinkDomain.NewLinker(links).Link(ctx, userID, profileID, rel)
	if err != nil {
		return nil, err
	}

	// 持久化关系记录
	if err := links.Create(ctx, newProfileLink); err != nil {
		return nil, err
	}

	return newProfileLink, nil
}
