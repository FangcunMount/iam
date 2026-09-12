package scopemigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"gorm.io/gorm"
)

// Snapshot captures IAM policy facts and QS company-membership facts independently.
// Writers must be paused and both sources rechecked at cutover; two databases
// cannot provide one atomic snapshot merely by using two read transactions.
type Snapshot struct {
	IAM rolemodel.State `json:"iam"`
	QS  BusinessFacts   `json:"qs"`
}

func (s Snapshot) Hash() string {
	s.QS.Operators = append([]OperatorFact(nil), s.QS.Operators...)
	s.QS.Stores = append([]StoreFact(nil), s.QS.Stores...)
	sort.Slice(s.QS.Operators, func(i, j int) bool { return s.QS.Operators[i].ID < s.QS.Operators[j].ID })
	sort.Slice(s.QS.Stores, func(i, j int) bool { return s.QS.Stores[i].ID < s.QS.Stores[j].ID })
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func LoadBusinessFacts(ctx context.Context, db *gorm.DB) (BusinessFacts, error) {
	facts := BusinessFacts{Operators: []OperatorFact{}, Stores: []StoreFact{}}
	if db == nil {
		return facts, fmt.Errorf("QS database required")
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("operators").Select("id, org_id, user_id, is_active AS active").Where("deleted_at IS NULL").Order("id ASC").Scan(&facts.Operators).Error; err != nil {
			return err
		}
		return tx.Table("actor_stores").Select("id, org_id, is_active AS active").Order("id ASC").Scan(&facts.Stores).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return facts, err
}
func LoadSnapshot(ctx context.Context, iam, qs *gorm.DB) (Snapshot, error) {
	var s Snapshot
	if iam == nil || qs == nil {
		return s, fmt.Errorf("IAM and QS databases required")
	}
	err := iam.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var e error; s.IAM, e = rolemodel.LoadState(ctx, tx); return e }, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return s, err
	}
	s.QS, err = LoadBusinessFacts(ctx, qs)
	return s, err
}
