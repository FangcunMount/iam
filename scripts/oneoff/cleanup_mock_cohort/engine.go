package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	assignmentapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policychange"
	userapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/identity/user"
	assignmentdomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	policyrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	rolerepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	sessiontasks "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/sessionrevocation"
	authzuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	identityuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/identity"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type engine struct {
	db      *gorm.DB
	redis   *redis.Client
	archive archive
	scope   iamScope
	journal *journal
	stager  event.Stager
	check   func(context.Context) error
}

func maintenanceServices(ctx context.Context, db *gorm.DB, catalogRaw []byte) (event.Stager, error) {
	c, e := eventcatalog.Parse(catalogRaw)
	if e != nil {
		return nil, e
	}
	return eventoutbox.NewMaintenanceStager(ctx, db, eventcatalog.NewCatalog(c), "standard")
}
func (e *engine) reader() sqlReader { return e.db.Statement.ConnPool }
func (e *engine) validate(ctx context.Context) error {
	if e.check != nil {
		if err := e.check(ctx); err != nil {
			return err
		}
	}
	id, err := readIdentity(ctx, e.reader())
	if err != nil {
		return err
	}
	id.RedisSHA, err = redisIdentity(ctx, e.redis)
	if err != nil {
		return err
	}
	if id != e.archive.Identity {
		return errors.New("IAM identity/clean state changed")
	}
	var protectedScope iamScope
	for _, im := range e.archive.Protected {
		switch im.Name {
		case "users":
			protectedScope.Users = tableIDs(im)
		case "profiles":
			protectedScope.Profiles = tableIDs(im)
		case "auth_login_identities":
			protectedScope.LoginIdentities = tableIDs(im)
		}
	}
	current, err := readScope(ctx, e.reader(), protectedScope)
	if err != nil {
		return err
	}
	if !imageRowsEqual(e.archive.Protected, current) {
		return errors.New("protected rows changed")
	}
	return nil
}

// Each transaction has a pre-COMMIT journal carrying both exact row images. It
// borrows the existing application transaction context, including event staging.
func (e *engine) mutate(ctx context.Context, phase string, before tableImage, fn func(context.Context, *gorm.DB) error) error {
	if len(before.Rows) > chunkSize {
		return errors.New("SQL chunk exceeds 1000 PK")
	}
	if err := e.validate(ctx); err != nil {
		return err
	}
	var after tableImage
	prepared := false
	err := dbmysql.NewUnitOfWork(e.db).WithinTransaction(ctx, func(txCtx context.Context) error {
		tx, err := dbmysql.RequireTx(txCtx)
		if err != nil {
			return err
		}
		cur, err := currentRowsLocked(txCtx, tx.Statement.ConnPool, before, before.Rows)
		if err != nil {
			return err
		}
		if !rowsEqual(cur.Rows, before.Rows) {
			return errors.New("preimage changed")
		}
		if err = fn(txCtx, tx); err != nil {
			return err
		}
		after, err = currentRows(txCtx, tx.Statement.ConnPool, before, before.Rows)
		if err != nil {
			return err
		}
		if err = e.journal.append(phase, before, after, "prepared"); err != nil {
			return err
		}
		prepared = true
		if e.check != nil {
			return e.check(txCtx)
		}
		return nil
	}, dbmysql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		if prepared {
			cur, readErr := currentRows(ctx, e.reader(), before, before.Rows)
			result := "readback_mixed"
			if readErr == nil {
				result = reconcileRows(cur.Rows, before.Rows, after.Rows)
			}
			jerr := e.journal.append(phase, before, after, result)
			return errors.Join(errors.New("transaction outcome requires read-only verification; no mutation replay"), err, readErr, jerr)
		}
		return err
	}
	cur, err := currentRows(ctx, e.reader(), before, before.Rows)
	if err != nil {
		return err
	}
	if !rowsEqual(cur.Rows, after.Rows) {
		return errors.New("post-commit row readback changed")
	}
	return e.journal.append(phase, before, after, "committed")
}
func (e *engine) apply(ctx context.Context) error {
	if err := e.validate(ctx); err != nil {
		return err
	}
	cur, err := readScope(ctx, e.reader(), e.scope)
	if err != nil {
		return err
	}
	if !imageRowsEqual(cur, e.archive.Tables) {
		return errors.New("capture is stale")
	}
	if err = validateExternal(ctx, e.reader(), e.scope); err != nil {
		return err
	}
	// This is a trusted offline maintenance call, admitted by the exact external
	// approval. It uses the same admin guard and durable policy-event path as the
	// existing management service; it does not invent an unguarded Domain API.
	ctx = management.WithAuthenticatedService(ctx, "admin")
	ctx = context.WithValue(ctx, requestctx.KeyUserID, "10001")
	commands := assignmentapp.NewCommandService(nil, rolerepo.NewRoleRepository(e.db), authzuow.NewUnitOfWork(e.db, nil, e.stager), nil, management.NewGuard(nil))
	assignments := lookup(e.archive.Tables, "authz_assignments")
	for _, rows := range rowChunks(assignments.Rows) {
		im := assignments
		im.Rows = rows
		if err = e.mutate(ctx, "revoke_assignments", im, func(txCtx context.Context, _ *gorm.DB) error {
			for _, r := range im.Rows {
				id, er := strconv.ParseUint(string(field(im, r, "id").Raw), 10, 64)
				if er != nil {
					return er
				}
				cmd, er := assignmentapp.NewRevokeByIDCommand(assignmentdomain.AssignmentID(meta.FromUint64(id)), "10001", "dedicated mock cohort cleanup")
				if er != nil {
					return er
				}
				if er = commands.RevokeByID(txCtx, cmd); er != nil {
					return er
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	changer := userapp.NewStatusChanger(identityuow.NewUnitOfWork(e.db))
	users := lookup(e.archive.Tables, "users")
	for _, rows := range rowChunks(users.Rows) {
		im := users
		im.Rows = rows
		if err = e.mutate(ctx, "deactivate_users", im, func(txCtx context.Context, _ *gorm.DB) error {
			for _, r := range im.Rows {
				id, er := meta.ParseID(string(field(im, r, "id").Raw))
				if er != nil {
					return er
				}
				if er = changer.Deactivate(txCtx, id); er != nil {
					return er
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if _, err = inspectSessions(ctx, e.redis, e.scope, e.archive.Tables, true); err != nil {
		return err
	}
	if _, err = inspectSessions(ctx, e.redis, e.scope, e.archive.Tables, false); err != nil {
		return err
	}
	// Complete only this cohort's durable revocation responsibilities, after
	// actual Redis readback. Other users' tasks are neither claimed nor changed.
	if err = e.completeSessionTasks(ctx); err != nil {
		return err
	}
	if err = e.journal.append("session_revocation", tableImage{}, tableImage{}, "sessions_revoked"); err != nil {
		return err
	}
	cur, err = readScope(ctx, e.reader(), e.scope)
	if err != nil {
		return err
	}
	if len(lookup(cur, "authz_assignments").Rows) != 0 {
		return errors.New("assignment revocation incomplete")
	}
	for _, im := range cur {
		for _, rows := range rowChunks(im.Rows) {
			part := im
			part.Rows = rows
			if err = e.mutate(ctx, "delete", part, func(txCtx context.Context, tx *gorm.DB) error { return deleteRows(txCtx, tx, part, part.Rows) }); err != nil {
				return err
			}
		}
	}
	if err = e.verify(ctx); err != nil {
		return err
	}
	return e.journal.append("apply", tableImage{}, tableImage{}, "complete")
}
func (e *engine) completeSessionTasks(ctx context.Context) error {
	for start := 0; start < len(e.scope.Users); start += chunkSize {
		ids := e.scope.Users[start:min(start+chunkSize, len(e.scope.Users))]
		p, a := idPredicate("user_id", ids)
		for {
			if err := e.validate(ctx); err != nil {
				return err
			}
			var tasks []sessiontasks.Task
			if err := e.db.WithContext(ctx).Where(p+" AND status <> 'completed'", a...).Order("task_id").Limit(chunkSize).Find(&tasks).Error; err != nil {
				return err
			}
			if len(tasks) == 0 {
				break
			}
			for _, task := range tasks {
				if task.Action != "revoke_all" {
					return errors.New("unknown target session responsibility")
				}
			}
			if err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				store := sessiontasks.NewStore(tx)
				for _, task := range tasks {
					r := tx.Exec("UPDATE identity_session_revocation_outbox SET status='processing',attempt_count=attempt_count+1,updated_at=? WHERE task_id=? AND user_id=? AND status=?", time.Now().UTC(), task.TaskID, task.UserID, task.Status)
					if r.Error != nil {
						return r.Error
					}
					if r.RowsAffected != 1 {
						return errors.New("revocation task changed")
					}
					if err := store.Complete(ctx, task.TaskID); err != nil {
						return err
					}
				}
				if e.check != nil {
					return e.check(ctx)
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
func (e *engine) verify(ctx context.Context) error {
	if err := e.validate(ctx); err != nil {
		return err
	}
	cur, err := readScope(ctx, e.reader(), e.scope)
	if err != nil {
		return err
	}
	if !schemaEqual(cur, e.archive.Tables) {
		return errors.New("schema changed")
	}
	for _, im := range cur {
		if len(im.Rows) != 0 {
			return fmt.Errorf("target rows remain in %s", im.Name)
		}
	}
	s, err := inspectSessions(ctx, e.redis, e.scope, e.archive.Tables, false)
	if err != nil {
		return err
	}
	for _, v := range s {
		if v.Status == "active" {
			return errors.New("target session remains active")
		}
	}
	return validateExternal(ctx, e.reader(), e.scope)
}
func (e *engine) restore(ctx context.Context, entries []journalEntry) error {
	if err := e.validate(ctx); err != nil {
		return err
	}
	known := knownPostimages(entries)
	originals := imageMap(e.archive.Tables)
	// Reject all existing foreign fixed-PK rows before irreversible session
	// revocation. Each write transaction still rechecks its locked preimage.
	for _, im := range e.archive.Tables {
		for _, rows := range rowChunks(im.Rows) {
			part := im
			part.Rows = rows
			cur, err := currentRows(ctx, e.reader(), part, part.Rows)
			if err != nil {
				return err
			}
			if err = checkRestoreRows(part, cur, originals, known); err != nil {
				return err
			}
		}
	}
	if err := e.validate(ctx); err != nil {
		return err
	}
	// A failure before Redis revocation must not leave a pending old revoke task
	// able to kill a newly created session after SQL restoration. Finish only the
	// original cohort's revoke responsibilities first; no historical session is
	// restored, even when the original User status is active.
	if _, err := inspectSessions(ctx, e.redis, e.scope, e.archive.Tables, true); err != nil {
		return err
	}
	if err := e.completeSessionTasks(ctx); err != nil {
		return err
	}
	// Restore parent tables first. Existing exact originals are idempotent;
	// only this operation's recorded postimages may be replaced. Foreign rows
	// and unique-key collisions stop the transaction, never ON DUPLICATE UPDATE.
	for i := len(e.archive.Tables) - 1; i >= 0; i-- {
		im := e.archive.Tables[i]
		for _, rows := range rowChunks(im.Rows) {
			part := im
			part.Rows = rows
			cur, err := currentRows(ctx, e.reader(), part, part.Rows)
			if err != nil {
				return err
			}
			if err = checkRestoreRows(part, cur, originals, known); err != nil {
				return err
			}
			if err = e.restoreChunk(ctx, part, cur); err != nil {
				return err
			}
		}
	}
	current, err := readScope(ctx, e.reader(), e.scope)
	if err != nil {
		return err
	}
	if !imageRowsEqual(current, e.archive.Tables) {
		return errors.New("restored values differ from captured originals")
	}
	// Restoring SQL never writes Redis session material, refresh tokens, or old
	// policy versions. A previously revoked session must remain unusable.
	s, err := inspectSessions(ctx, e.redis, e.scope, e.archive.Tables, false)
	if err != nil {
		return err
	}
	for _, v := range s {
		if v.Status == "active" {
			return errors.New("restore found an active historical session")
		}
	}
	return e.journal.append("restore", tableImage{}, tableImage{}, "complete")
}

func checkRestoreRows(im, current tableImage, originals map[string]row, known map[string][]row) error {
	for _, r := range current.Rows {
		k := im.Name + ":" + string(field(im, r, "id").Raw)
		if reflect.DeepEqual(r, originals[k]) {
			continue
		}
		allowed := false
		for _, post := range known[k] {
			allowed = allowed || reflect.DeepEqual(post, r)
		}
		if !allowed {
			return errors.New("restore conflict: row is neither original nor this operation's postimage")
		}
	}
	return nil
}

func (e *engine) restoreChunk(ctx context.Context, target, current tableImage) error {
	// Missing rows need a target-ID preimage, while mutate normally obtains its
	// selector from Before. Retain absent IDs as the target argument separately.
	if err := e.validate(ctx); err != nil {
		return err
	}
	prepared := false
	err := dbmysql.NewUnitOfWork(e.db).WithinTransaction(ctx, func(txCtx context.Context) error {
		tx, err := dbmysql.RequireTx(txCtx)
		if err != nil {
			return err
		}
		now, err := currentRowsLocked(txCtx, tx.Statement.ConnPool, target, target.Rows)
		if err != nil {
			return err
		}
		if !rowsEqual(now.Rows, current.Rows) {
			return errors.New("restore preimage changed")
		}
		changed := false
		currentMap := imageMap([]tableImage{current})
		for _, r := range target.Rows {
			old := currentMap[current.Name+":"+string(r.Values[0].Raw)]
			if reflect.DeepEqual(old, r) {
				continue
			}
			changed = true
			if len(old.Values) > 0 {
				if err = deleteRows(txCtx, tx, current, []row{old}); err != nil {
					return err
				}
			}
			if err = insertRow(txCtx, tx, target, r); err != nil {
				return err
			}
		}
		if target.Name == "authz_assignments" && changed {
			version, er := policyrepo.NewPolicyVersionRepository(tx).Increment(txCtx, "10001", "restore mock cohort assignment preimages")
			if er != nil {
				return er
			}
			if er = policychange.StagePolicyVersionChanged(txCtx, e.stager, version); er != nil {
				return er
			}
		}
		got, err := currentRows(txCtx, tx.Statement.ConnPool, target, target.Rows)
		if err != nil {
			return err
		}
		if !rowsEqual(got.Rows, target.Rows) {
			return errors.New("restore exact row readback failed")
		}
		if err = e.journal.append("restore_rows", current, target, "prepared"); err != nil {
			return err
		}
		prepared = true
		if e.check != nil {
			return e.check(txCtx)
		}
		return nil
	}, dbmysql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		if prepared {
			got, re := currentRows(ctx, e.reader(), target, target.Rows)
			result := "readback_mixed"
			if re == nil {
				result = reconcileRows(got.Rows, current.Rows, target.Rows)
			}
			return errors.Join(err, re, e.journal.append("restore_rows", current, target, result), errors.New("restore commit outcome retained; no mutation replay"))
		}
		return err
	}
	got, err := currentRows(ctx, e.reader(), target, target.Rows)
	if err != nil {
		return err
	}
	if !rowsEqual(got.Rows, target.Rows) {
		return errors.New("restore committed readback differs")
	}
	return e.journal.append("restore_rows", current, target, "committed")
}
