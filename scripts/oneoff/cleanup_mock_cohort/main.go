package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/FangcunMount/iam/v5/pkg/version"
	driver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type inputFile struct{ path, sha string }
type options struct {
	action                                                                                      string
	group, owner, protected, archive, approval, handoff, frozen, catalog, journal, priorJournal inputFile
	qsCapture, qsApply, qsVerify, messages                                                      inputFile
	qsClearance, qsClearanceExecution                                                           inputFile
	deadline                                                                                    time.Duration
}

func operationContext(o options, a approval) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(o.deadline)
	if o.action == "apply" || o.action == "restore" {
		if a.Expires.Before(deadline) {
			deadline = a.Expires
		}
		if a.Deadline.Before(deadline) {
			deadline = a.Deadline
		}
	}
	return context.WithDeadline(context.Background(), deadline)
}

type receipt struct {
	Format       int      `json:"format_version"`
	Kind         string   `json:"kind"`
	Action       string   `json:"action"`
	Source       string   `json:"compiled_source"`
	Operation    string   `json:"operation_id"`
	GroupSHA     string   `json:"group_manifest_sha256"`
	ArchiveSHA   string   `json:"archive_sha256"`
	JournalSHA   string   `json:"journal_sha256"`
	Identity     identity `json:"identity"`
	Counts       counts   `json:"iam_counts"`
	Complete     bool     `json:"complete"`
	SQLClosed    bool     `json:"sql_closed"`
	RedisClosed  bool     `json:"redis_closed"`
	GateReleased bool     `json:"gate_released"`
	Error        string   `json:"error_category"`
}

func main() { os.Exit(serve(os.Args[1:], os.Stdout, os.Getenv)) }
func parseArgs(args []string) (options, error) {
	var o options
	f := flag.NewFlagSet("cleanup_mock_cohort", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&o.action, "action", "plan", "plan|capture|apply|verify|restore")
	f.DurationVar(&o.deadline, "deadline", 25*time.Minute, "bounded operation deadline (max 55m)")
	for _, x := range []struct {
		name string
		v    *inputFile
	}{{"group", &o.group}, {"owner-confirmation", &o.owner}, {"protected", &o.protected}, {"archive", &o.archive}, {"approval", &o.approval}, {"qs-handoff", &o.handoff}, {"frozen", &o.frozen}, {"event-catalog", &o.catalog}, {"journal", &o.journal}, {"prior-journal", &o.priorJournal}, {"qs-capture", &o.qsCapture}, {"qs-apply", &o.qsApply}, {"qs-verify", &o.qsVerify}, {"qs-clearance", &o.qsClearance}, {"qs-clearance-execution", &o.qsClearanceExecution}, {"messages-settled", &o.messages}} {
		f.StringVar(&x.v.path, x.name+"-file", "", "private absolute file")
		f.StringVar(&x.v.sha, x.name+"-sha256", "", "original raw SHA256")
	}
	if err := f.Parse(args); err != nil {
		return o, errors.New("arguments rejected")
	}
	if f.NArg() != 0 {
		return o, errors.New("positional arguments rejected")
	}
	switch o.action {
	case "plan", "capture", "apply", "verify", "restore":
	default:
		return o, errors.New("unknown action")
	}
	if o.deadline <= 0 || o.deadline > 55*time.Minute {
		return o, errors.New("deadline outside 1ns..55m")
	}
	clearanceProvided := false
	f.Visit(func(v *flag.Flag) {
		switch v.Name {
		case "qs-clearance-file", "qs-clearance-sha256", "qs-clearance-execution-file", "qs-clearance-execution-sha256":
			clearanceProvided = true
		}
	})
	mode, err := qsClearanceMode(o)
	if err != nil {
		return o, err
	}
	if clearanceProvided && !mode {
		return o, errors.New("both QS clearance originals and raw SHA required")
	}
	return o, nil
}
func serve(args []string, out io.Writer, getenv func(string) string) int {
	o, err := parseArgs(args)
	r := receipt{Format: 1, Kind: "mock_cohort_iam_receipt", Action: o.action, Source: version.GitCommit}
	if err == nil {
		err = execute(o, &r, getenv)
	}
	if err != nil {
		r.Error = "operation_incomplete"
	}
	if encErr := json.NewEncoder(out).Encode(r); encErr != nil {
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}
func load(in inputFile) ([]byte, error) { return readPrivate(in.path, in.sha, maxDocument) }
func execute(o options, r *receipt, getenv func(string) string) (retErr error) {
	clearanceMode, err := qsClearanceMode(o)
	if err != nil {
		return err
	}
	groupRaw, err := load(o.group)
	if err != nil {
		return err
	}
	ownerRaw, err := load(o.owner)
	if err != nil {
		return err
	}
	protectedRaw, err := load(o.protected)
	if err != nil {
		return err
	}
	g, s, err := validateGroup(groupRaw, ownerRaw, protectedRaw)
	if err != nil {
		return err
	}
	r.Operation = g.Operation
	r.GroupSHA = rawSHA(groupRaw)
	r.Counts = s.counts()
	var p protection
	if err = strictJSON(protectedRaw, &p); err != nil {
		return err
	}
	var owner ownerConfirmation
	if err = strictJSON(ownerRaw, &owner); err != nil {
		return err
	}
	p.IAM.Users = appendUnique(p.IAM.Users, "10001", "618855329043591726")
	p.IAM.Profiles = appendUnique(p.IAM.Profiles, "632367179001508398", "636374014138266158")
	var a archive
	var handoff qsHandoff
	var clearance qsClearanceReceipt
	var handoffFrozenSHA string
	var app approval
	var freeze freezeReceipt
	var catalogRaw []byte
	var entries []journalEntry
	write := o.action == "apply" || o.action == "restore"
	if o.action == "capture" && !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(r.Source) {
		return errors.New("capture requires compiled source identity")
	}
	if o.action != "plan" && o.action != "capture" {
		ar, er := load(o.archive)
		if er != nil {
			return er
		}
		if er = strictJSON(ar, &a); er != nil {
			return er
		}
		if er = validateArchive(a, g, r.GroupSHA); er != nil {
			return er
		}
		if er = validateClosure(a.Tables, s); er != nil {
			return er
		}
		r.ArchiveSHA = rawSHA(ar)
	}
	if write {
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(r.Source) {
			return errors.New("compiled source required for mutation")
		}
		hRaw, er := load(o.handoff)
		if er != nil {
			return er
		}
		if clearanceMode {
			clearance, handoffFrozenSHA, er = loadQSClearance(o, hRaw, g, s, r.GroupSHA, time.Now())
			if er != nil {
				return er
			}
		} else {
			handoff, er = validateHandoff(hRaw, g, s, r.GroupSHA)
			if er != nil {
				return er
			}
			handoffFrozenSHA = handoff.FrozenSHA
			materialSHA, qsSource := "", ""
			for i, x := range []struct {
				in  inputFile
				sha string
			}{{o.qsCapture, handoff.CaptureSHA}, {o.qsApply, handoff.ApplySHA}, {o.qsVerify, handoff.VerifySHA}, {o.messages, handoff.MessagesSHA}} {
				if x.in.sha != x.sha {
					return errors.New("QS original evidence SHA differs")
				}
				raw, er := load(x.in)
				if er != nil {
					return er
				}
				if i < 3 {
					action := []string{"capture", "apply", "verify"}[i]
					qr, er := validateQSAction(raw, action, g.Operation, r.GroupSHA)
					if er != nil {
						return er
					}
					if materialSHA == "" {
						materialSHA = qr.MaterialSHA
						qsSource = qr.Source
					}
					if materialSHA != qr.MaterialSHA || qsSource != qr.Source {
						return errors.New("QS action material differs")
					}
				} else {
					var doc any
					if er = json.Unmarshal(raw, &doc); er != nil {
						return er
					}
				}
			}
		}
		fRaw, er := load(o.frozen)
		if er != nil {
			return er
		}
		if o.frozen.sha != handoffFrozenSHA {
			return errors.New("frozen handoff differs")
		}
		if er = strictJSON(fRaw, &freeze); er != nil {
			return er
		}
		if freeze.Format != 1 || freeze.Kind != "mock_cohort_frozen" || freeze.Operation != g.Operation || freeze.GroupSHA != r.GroupSHA || !freeze.IAMStopped || !freeze.QSStopped || !freeze.ReplaySettled || !freeze.Reload {
			return errors.New("frozen/replay/projection responsibility incomplete")
		}
		appRaw, er := load(o.approval)
		if er != nil {
			return er
		}
		if er = strictJSON(appRaw, &app); er != nil {
			return er
		}
		if app.CatalogSHA != o.catalog.sha || !digestPattern.MatchString(app.CatalogSHA) || app.PriorJournalSHA != o.priorJournal.sha {
			return errors.New("catalog/journal approval binding rejected")
		}
		if er = validateApproval(app, o.action, r.Source, g.Operation, r.GroupSHA, r.ArchiveSHA, rawSHA(hRaw), rawSHA(fRaw), a.Identity, time.Now()); er != nil {
			return er
		}
		if clearanceMode {
			if er = validateQSClearanceApproval(clearance, app, time.Now()); er != nil {
				return er
			}
		}
		catalogRaw, err = load(o.catalog)
		if err != nil {
			return err
		}
		if o.action == "restore" {
			raw, er := load(o.priorJournal)
			if er != nil {
				return er
			}
			entries, er = parseJournal(raw, g.Operation, r.ArchiveSHA)
			if er != nil {
				return er
			}
		}
	}
	// All material/approval/handoff admission above precedes reading credentials.
	dsn, err := driver.ParseDSN(getenv("IAM_MOCK_CLEANUP_MYSQL_DSN"))
	if err != nil || dsn.DBName == "" {
		return errors.New("explicit IAM MySQL DSN required")
	}
	dsn.ParseTime = true
	dsn.MultiStatements = false
	dsn.Timeout = 5 * time.Second
	dsn.ReadTimeout = 15 * time.Second
	dsn.WriteTimeout = 15 * time.Second
	dsn.Loc = time.UTC
	db, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		return errors.New("MySQL open failed")
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		e := db.Close()
		r.SQLClosed = e == nil
		retErr = errors.Join(retErr, e)
		if retErr != nil {
			r.Complete = false
		}
	}()
	ctx, cancel := operationContext(o, app)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return errors.New("MySQL connection failed")
	}
	defer func() { retErr = errors.Join(retErr, conn.Close()) }()
	id, err := readIdentity(ctx, conn)
	if err != nil {
		return err
	}
	r.Identity = id
	redisOpt, err := redis.ParseURL(getenv("IAM_MOCK_CLEANUP_REDIS_URL"))
	if err != nil {
		return errors.New("explicit IAM Redis URL required")
	}
	redisOpt.DialTimeout = 5 * time.Second
	redisOpt.ReadTimeout = 5 * time.Second
	redisOpt.WriteTimeout = 5 * time.Second
	rc := redis.NewClient(redisOpt)
	defer func() {
		e := rc.Close()
		r.RedisClosed = e == nil
		retErr = errors.Join(retErr, e)
		if retErr != nil {
			r.Complete = false
		}
	}()
	id.RedisSHA, err = redisIdentity(ctx, rc)
	if err != nil {
		return err
	}
	r.Identity = id
	gate := "iam-mock-cohort:" + rawSHA([]byte(id.Database + "\x00" + id.ServerUUID))[:40]
	var locked int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", gate).Scan(&locked); err != nil || locked != 1 {
		return errors.New("maintenance gate unavailable")
	}
	defer func() {
		releaseCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		var released int
		e := conn.QueryRowContext(releaseCtx, "SELECT RELEASE_LOCK(?)", gate).Scan(&released)
		r.GateReleased = e == nil && released == 1
		if !r.GateReleased {
			retErr = errors.Join(retErr, errors.New("gate release unproven"))
			r.Complete = false
		}
	}()
	if o.action == "plan" || o.action == "capture" {
		tx, er := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
		if er != nil {
			return er
		}
		defer func() { _ = tx.Rollback() }()
		images, er := readScope(ctx, tx, s)
		if er != nil {
			return er
		}
		if er = validateClosure(images, s); er != nil {
			return er
		}
		if er = validateAssignmentOrgs(images, owner.AssignmentOrgs); er != nil {
			return er
		}
		if er = validateExternal(ctx, tx, s); er != nil {
			return er
		}
		protected, er := readScope(ctx, tx, p.IAM)
		if er != nil {
			return er
		}
		id2, er := readIdentity(ctx, tx)
		id2.RedisSHA = id.RedisSHA
		if er != nil || id2 != id {
			return errors.New("capture identity changed")
		}
		if er = tx.Commit(); er != nil {
			return er
		}
		if o.action == "capture" {
			a = archive{1, "mock_cohort_iam_archive", g.Operation, r.Source, r.GroupSHA, id, images, protected}
			raw := encode(a)
			if len(raw) > maxDocument {
				return errors.New("archive document bound exceeded; split fixed cohort")
			}
			if er = writeExclusive(o.archive.path, raw); er != nil {
				return er
			}
			r.ArchiveSHA = rawSHA(raw)
		}
		r.Complete = true
		return nil
	}
	if id != a.Identity {
		return errors.New("archive database identity differs")
	}
	if err = validateAssignmentOrgs(a.Tables, owner.AssignmentOrgs); err != nil {
		return err
	}
	gdb, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableAutomaticPing: true})
	if err != nil {
		return errors.New("transaction adapter unavailable")
	}
	e := engine{db: gdb, redis: rc, archive: a, scope: s}
	if o.action == "verify" {
		err = e.verify(ctx)
		r.Complete = err == nil
		return err
	}
	stager, err := maintenanceServices(ctx, gdb, catalogRaw)
	if err != nil {
		return err
	}
	e.stager = stager
	// Revalidate immutable inputs and the original absolute deadline at every
	// transaction boundary and immediately before the outer COMMIT.
	e.check = func(checkCtx context.Context) error {
		if er := checkCtx.Err(); er != nil {
			return er
		}
		if er := validateApproval(app, o.action, r.Source, g.Operation, r.GroupSHA, r.ArchiveSHA, o.handoff.sha, o.frozen.sha, a.Identity, time.Now()); er != nil {
			return er
		}
		if clearanceMode {
			if er := validateQSClearanceApproval(clearance, app, time.Now()); er != nil {
				return er
			}
		}
		inputs := []inputFile{o.group, o.owner, o.protected, o.archive, o.approval, o.handoff, o.frozen, o.catalog, o.messages, o.qsCapture, o.qsApply}
		if clearanceMode {
			inputs = append(inputs, o.qsClearance, o.qsClearanceExecution)
		} else {
			inputs = append(inputs, o.qsVerify)
		}
		for _, in := range inputs {
			if _, er := load(in); er != nil {
				return er
			}
		}
		return nil
	}
	if err = e.check(ctx); err != nil {
		return err
	}
	if o.action == "apply" {
		claim := o.archive.path + ".apply-claim"
		if err = writeExclusive(claim, encode(map[string]string{"operation_id": g.Operation, "approval_sha256": o.approval.sha, "archive_sha256": r.ArchiveSHA})); err != nil {
			return errors.New("apply already claimed; inspect or restore with new explicit approval")
		}
	}
	j, err := newJournal(o.journal.path, g.Operation, r.ArchiveSHA)
	if err != nil {
		return err
	}
	e.journal = j
	defer func() {
		closeErr := j.f.Close()
		raw, readErr := os.ReadFile(o.journal.path)
		if readErr == nil {
			r.JournalSHA = rawSHA(raw)
		}
		retErr = errors.Join(retErr, closeErr, readErr)
		if retErr != nil {
			r.Complete = false
		}
	}()
	if o.action == "apply" {
		err = e.apply(ctx)
	} else {
		err = e.restore(ctx, entries)
	}
	r.Complete = err == nil
	return err
}
func appendUnique(ids []string, more ...string) []string {
	out := append([]string{}, ids...)
	for _, id := range more {
		if !contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
func validateArchive(a archive, g groupManifest, sha string) error {
	if a.Format != 1 || a.Kind != "mock_cohort_iam_archive" || a.Operation != g.Operation || a.GroupSHA != sha || a.Identity.Version != 40 || a.Identity.Dirty || len(a.Tables) != len(tableOrder) || len(a.Protected) != len(tableOrder) {
		return errors.New("archive binding rejected")
	}
	if !schemaEqual(a.Tables, a.Protected) {
		return errors.New("archive schema differs between target and protected rows")
	}
	for i, name := range tableOrder {
		if a.Tables[i].Name != name || a.Protected[i].Name != name {
			return errors.New("archive table order rejected")
		}
		for _, im := range []tableImage{a.Tables[i], a.Protected[i]} {
			if len(im.Columns) == 0 || im.Columns[0].Name != "id" || im.Rows == nil || im.DDL == "" {
				return errors.New("archive shape rejected")
			}
			seen := map[string]bool{}
			for _, r := range im.Rows {
				if len(r.Values) != len(im.Columns) {
					return errors.New("archive row width rejected")
				}
				id := string(r.Values[0].Raw)
				if r.Values[0].Null || seen[id] || validIDs([]string{id}) != nil {
					return errors.New("archive primary key rejected")
				}
				seen[id] = true
				for _, c := range r.Values {
					if c.Null && len(c.Raw) != 0 {
						return errors.New("noncanonical NULL cell")
					}
				}
			}
		}
	}
	return nil
}
func validateApproval(a approval, action, source, op, group, archiveSHA, handoff, frozen string, id identity, now time.Time) error {
	if a.Format != 1 || a.Kind != "mock_cohort_iam_approval" || a.Action != action || a.Source != source || a.Operation != op || a.GroupSHA != group || a.ArchiveSHA != archiveSHA || a.HandoffSHA != handoff || a.FrozenSHA != frozen || a.Identity != id {
		return errors.New("current approval binding rejected")
	}
	limit := 30 * time.Minute
	if action == "restore" {
		limit = 60 * time.Minute
	}
	if a.Issued.IsZero() || now.Before(a.Issued) || !now.Before(a.Expires) || !now.Before(a.Deadline) || a.Expires.After(a.Deadline) || a.Deadline.Sub(a.Issued) > limit {
		return errors.New("current approval time rejected")
	}
	return nil
}
