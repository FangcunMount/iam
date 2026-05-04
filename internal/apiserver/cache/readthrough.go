package cache

import (
	"context"
	"errors"
	"time"
)

var (
	ErrMissingReadThroughGet  = errors.New("cache read-through get function is required")
	ErrMissingReadThroughLoad = errors.New("cache read-through load function is required")
	ErrMissingReadThroughLock = errors.New("cache locked read-through lock function is required")
	ErrLockNotAcquired        = errors.New("cache lock not acquired")
)

// ReadThroughOptions 描述一次最小缓存读穿透流程。
type ReadThroughOptions[T any] struct {
	Get            func(context.Context) (*T, error)
	Valid          func(*T) bool
	Load           func(context.Context) (*T, error)
	TTL            func(*T) time.Duration
	Set            func(context.Context, *T, time.Duration) error
	IgnoreGetError bool
}

// LockedReadThroughOptions 在 ReadThrough 基础上增加短租约协调。
type LockedReadThroughOptions[T any] struct {
	ReadThroughOptions[T]
	Lock          func(context.Context) (ok bool, unlock func(), err error)
	RereadUsable  func(*T) bool
	LockMissError error
}

// ReadThrough 执行缓存 hit -> load -> set 的最小流程。
func ReadThrough[T any](ctx context.Context, opts ReadThroughOptions[T]) (*T, error) {
	if opts.Get == nil {
		return nil, ErrMissingReadThroughGet
	}
	if opts.Load == nil {
		return nil, ErrMissingReadThroughLoad
	}
	if cached, err := opts.Get(ctx); err == nil {
		if cached != nil && isValid(cached, opts.Valid) {
			return cached, nil
		}
	} else if !opts.IgnoreGetError {
		return nil, err
	}

	return loadAndSet(ctx, opts)
}

// LockedReadThrough 执行带短租约的缓存读穿透流程。
func LockedReadThrough[T any](ctx context.Context, opts LockedReadThroughOptions[T]) (*T, error) {
	if opts.Get == nil {
		return nil, ErrMissingReadThroughGet
	}
	if opts.Load == nil {
		return nil, ErrMissingReadThroughLoad
	}
	if opts.Lock == nil {
		return nil, ErrMissingReadThroughLock
	}
	if cached, err := opts.Get(ctx); err == nil {
		if cached != nil && isValid(cached, opts.Valid) {
			return cached, nil
		}
	} else if !opts.IgnoreGetError {
		return nil, err
	}

	ok, unlock, err := opts.Lock(ctx)
	if err != nil {
		return nil, err
	}
	if ok {
		if unlock != nil {
			defer unlock()
		}
		return loadAndSet(ctx, opts.ReadThroughOptions)
	}

	if cached, err := opts.Get(ctx); err == nil {
		if cached != nil && isRereadUsable(cached, opts.RereadUsable, opts.Valid) {
			return cached, nil
		}
	} else if !opts.IgnoreGetError {
		return nil, err
	}
	if opts.LockMissError != nil {
		return nil, opts.LockMissError
	}
	return nil, ErrLockNotAcquired
}

func loadAndSet[T any](ctx context.Context, opts ReadThroughOptions[T]) (*T, error) {
	loaded, err := opts.Load(ctx)
	if err != nil {
		return nil, err
	}
	if loaded == nil {
		return nil, nil
	}
	if opts.Set != nil {
		if err := opts.Set(ctx, loaded, ttlFor(loaded, opts.TTL)); err != nil {
			return nil, err
		}
	}
	return loaded, nil
}

func isValid[T any](value *T, valid func(*T) bool) bool {
	if value == nil {
		return false
	}
	if valid == nil {
		return true
	}
	return valid(value)
}

func isRereadUsable[T any](value *T, usable func(*T) bool, valid func(*T) bool) bool {
	if value == nil {
		return false
	}
	if usable != nil {
		return usable(value)
	}
	return isValid(value, valid)
}

func ttlFor[T any](value *T, ttl func(*T) time.Duration) time.Duration {
	if ttl == nil || value == nil {
		return 0
	}
	return ttl(value)
}
