package registration

import (
	"context"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/logger"
	childapp "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	guardapp "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	childdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	guarddomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type childRegistrationService struct {
	uow uow.UnitOfWork
}

// NewChildRegistrationService 创建跨 child/guardianship 的组合注册服务。
func NewChildRegistrationService(uow uow.UnitOfWork) ChildRegistrationService {
	return &childRegistrationService{uow: uow}
}

func (s *childRegistrationService) RegisterChildWithGuardian(ctx context.Context, dto RegisterChildWithGuardianDTO) (*RegisterChildWithGuardianResult, error) {
	l := logger.L(ctx)
	var result *RegisterChildWithGuardianResult

	l.Debugw("注册儿童并建立监护关系",
		"action", logger.ActionRegister,
		"resource", logger.ResourceChild,
		"user_id", dto.UserID,
		"child_name", dto.Name,
		"relation", dto.Relation,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		userID, err := parseUserID(dto.UserID)
		if err != nil {
			return err
		}

		newChild, err := buildChildEntity(ctx, tx, dto)
		if err != nil {
			return err
		}
		if err := tx.Children.Create(txCtx, newChild); err != nil {
			return err
		}

		manager := guarddomain.NewManagerService(tx.Guardianships, tx.Children, tx.Users)
		newGuardianship, err := manager.AddGuardian(ctx, userID, newChild.ID, guarddomain.ParseRelation(dto.Relation))
		if err != nil {
			return err
		}
		if err := tx.Guardianships.Create(txCtx, newGuardianship); err != nil {
			return err
		}

		result = &RegisterChildWithGuardianResult{
			Child:        childToResult(newChild),
			Guardianship: guardianshipToResult(newGuardianship, newChild),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func buildChildEntity(ctx context.Context, tx uow.TxRepositories, dto RegisterChildWithGuardianDTO) (*childdomain.Child, error) {
	name := strings.TrimSpace(dto.Name)
	validator := childdomain.NewValidator(tx.Children)
	gender := input.ParseGender(dto.Gender)
	birthday := input.ParseBirthday(strings.TrimSpace(dto.Birthday))
	if err := validator.ValidateRegister(ctx, name, gender, birthday); err != nil {
		return nil, err
	}

	options := []childdomain.ChildOption{
		childdomain.WithGender(gender),
		childdomain.WithBirthday(birthday),
	}
	if strings.TrimSpace(dto.IDCard) != "" {
		idCard, err := input.ParseIDCard(name, strings.TrimSpace(dto.IDCard))
		if err != nil {
			return nil, err
		}
		options = append(options, childdomain.WithIDCard(idCard))
	}

	newChild, err := childdomain.NewChild(name, options...)
	if err != nil {
		return nil, err
	}

	if dto.Height != nil || dto.Weight != nil {
		height := newChild.Height
		if dto.Height != nil {
			parsedHeight, err := input.ParseHeightCm(*dto.Height)
			if err != nil {
				return nil, err
			}
			height = parsedHeight
		}

		weight := newChild.Weight
		if dto.Weight != nil {
			parsedWeight, err := input.ParseWeightGrams(*dto.Weight)
			if err != nil {
				return nil, err
			}
			weight = parsedWeight
		}

		newChild.UpdateHeightWeight(height, weight)
	}

	return newChild, nil
}

func childToResult(child *childdomain.Child) *childapp.ChildResult {
	if child == nil {
		return nil
	}

	return &childapp.ChildResult{
		ID:       child.ID.String(),
		Name:     child.Name,
		IDCard:   child.IDCard.String(),
		Gender:   child.Gender.Value(),
		Birthday: child.Birthday.String(),
		Height:   input.HeightCm(child.Height),
		Weight:   input.WeightGrams(child.Weight),
	}
}

func guardianshipToResult(guardianship *guarddomain.Guardianship, child *childdomain.Child) *guardapp.GuardianshipResult {
	if guardianship == nil {
		return nil
	}

	result := &guardapp.GuardianshipResult{
		ID:            guardianship.ID.Uint64(),
		UserID:        guardianship.User.String(),
		ChildID:       guardianship.Child.String(),
		Relation:      string(guardianship.Rel),
		EstablishedAt: guardianship.EstablishedAt.Format(time.RFC3339),
	}
	if guardianship.RevokedAt != nil && !guardianship.RevokedAt.IsZero() {
		result.RevokedAt = guardianship.RevokedAt.Format(time.RFC3339)
	}
	if child != nil {
		result.ChildName = child.Name
		result.ChildGender = child.Gender.Value()
		result.ChildBirthday = child.Birthday.String()
	}

	return result
}

func parseUserID(userID string) (meta.ID, error) {
	return input.ParseUserID(strings.TrimSpace(userID))
}
