package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/container/platform"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"gorm.io/gorm"
)

type maintenanceCompletionError struct{ BusinessErr, ShutdownErr error }

func (e *maintenanceCompletionError) Error() string {
	if e.BusinessErr == nil {
		return "maintenance business completed; transaction messaging shutdown incomplete"
	}
	return "maintenance business outcome requires inspection; transaction messaging shutdown incomplete"
}
func (e *maintenanceCompletionError) Unwrap() []error {
	if e.BusinessErr == nil {
		return []error{e.ShutdownErr}
	}
	return []error{e.BusinessErr, e.ShutdownErr}
}
func maintenanceBusinessSucceeded(err error) bool {
	var completion *maintenanceCompletionError
	return err == nil || (errors.As(err, &completion) && completion.BusinessErr == nil)
}

type maintenanceOwner interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type maintenanceStartError struct{ Cause, ShutdownErr error }

func (e *maintenanceStartError) Error() string {
	if e.ShutdownErr != nil {
		return "transaction messaging startup failed; client shutdown incomplete"
	}
	return "transaction messaging startup failed before maintenance admission"
}
func (e *maintenanceStartError) Unwrap() []error {
	if e.ShutdownErr == nil {
		return []error{e.Cause}
	}
	return []error{e.Cause, e.ShutdownErr}
}
func runMaintenanceOwner(ctx context.Context, stager event.Stager, owner maintenanceOwner, fn func(event.Stager) error) (result error) {
	if err := owner.Start(ctx); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return &maintenanceStartError{err, owner.Stop(cleanup)}
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if shutdownErr := owner.Stop(cleanup); shutdownErr != nil {
			result = &maintenanceCompletionError{result, shutdownErr}
		}
	}()
	return fn(stager)
}

func loadMaintenanceMessaging(path string) (options.MessagingOptions, error) {
	var config options.MessagingOptions
	if path == "" {
		return config, errors.New("transaction mode requires --transaction-messaging-config")
	}
	file, err := os.Open(path)
	if err != nil {
		return config, errors.New("cannot open transaction messaging config")
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64*1024 {
		return config, errors.New("transaction messaging config must be a regular file of at most64KiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 64*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&config) != nil {
		return config, errors.New("invalid explicit transaction messaging config")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return config, errors.New("transaction messaging config has trailing data")
	}
	if config.SelectedProvider() != "rocketmq" || !config.PublishEnabled || len(config.Transactions) != 1 {
		return config, errors.New("transaction maintenance requires explicit RocketMQ writer admission and one IAM owner")
	}
	if err = config.Validate(); err != nil {
		return config, fmt.Errorf("invalid transaction messaging options: %w", err)
	}
	return config, nil
}
func withMaintenanceStager(ctx context.Context, db *gorm.DB, catalog *eventcatalog.Catalog, mode, path string, fn func(event.Stager) error) error {
	if mode != "transaction" {
		return errors.New("transaction maintenance is required; legacy and standard publishing profiles are retired")
	}

	config, err := loadMaintenanceMessaging(path)
	if err != nil {
		return err
	}
	owner, err := platform.NewTransactionEventing(platform.EventingDeps{DB: db, Messaging: config}, catalog, "maintenance:uid:"+strconv.Itoa(os.Geteuid()))
	if err != nil {
		return err
	}
	return runMaintenanceOwner(ctx, owner.Writer, owner, fn)
}

// Incomplete shutdown still owns real SDK DB calls. A one-shot CLI reports the
// pending crash/recovery obligation; it must not close that pool as if drained.
func closeMaintenancePool(pool interface{ Close() error }, protocolErr error) {
	if errors.Is(protocolErr, transaction.ErrShutdownIncomplete) {
		return
	}
	_ = pool.Close()
}
