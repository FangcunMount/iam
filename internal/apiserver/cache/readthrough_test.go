package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

type readThroughValue struct {
	token string
	valid bool
}

func TestReadThroughHit(t *testing.T) {
	loadCalled := false
	got, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Get: func(context.Context) (*readThroughValue, error) {
			return &readThroughValue{token: "cached", valid: true}, nil
		},
		Valid: func(value *readThroughValue) bool {
			return value.valid
		},
		Load: func(context.Context) (*readThroughValue, error) {
			loadCalled = true
			return &readThroughValue{token: "loaded"}, nil
		},
	})
	if err != nil {
		t.Fatalf("ReadThrough() error = %v", err)
	}
	if got == nil || got.token != "cached" {
		t.Fatalf("ReadThrough() token = %#v, want cached", got)
	}
	if loadCalled {
		t.Fatalf("Load called on cache hit")
	}
}

func TestReadThroughRequiresGet(t *testing.T) {
	_, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Load: func(context.Context) (*readThroughValue, error) {
			return &readThroughValue{token: "loaded"}, nil
		},
	})
	if !errors.Is(err, ErrMissingReadThroughGet) {
		t.Fatalf("ReadThrough() error = %v, want %v", err, ErrMissingReadThroughGet)
	}
}

func TestReadThroughRequiresLoad(t *testing.T) {
	_, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Get: func(context.Context) (*readThroughValue, error) {
			return nil, nil
		},
	})
	if !errors.Is(err, ErrMissingReadThroughLoad) {
		t.Fatalf("ReadThrough() error = %v, want %v", err, ErrMissingReadThroughLoad)
	}
}

func TestReadThroughMissLoadsAndSets(t *testing.T) {
	var setTTL time.Duration
	got, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Get: func(context.Context) (*readThroughValue, error) {
			return nil, nil
		},
		Load: func(context.Context) (*readThroughValue, error) {
			return &readThroughValue{token: "loaded"}, nil
		},
		TTL: func(*readThroughValue) time.Duration {
			return time.Minute
		},
		Set: func(_ context.Context, value *readThroughValue, ttl time.Duration) error {
			if value.token != "loaded" {
				t.Fatalf("Set value = %#v, want loaded", value)
			}
			setTTL = ttl
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ReadThrough() error = %v", err)
	}
	if got == nil || got.token != "loaded" {
		t.Fatalf("ReadThrough() token = %#v, want loaded", got)
	}
	if setTTL != time.Minute {
		t.Fatalf("Set ttl = %v, want %v", setTTL, time.Minute)
	}
}

func TestReadThroughGetErrorPropagatesByDefault(t *testing.T) {
	getErr := errors.New("get failed")
	_, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Get: func(context.Context) (*readThroughValue, error) {
			return nil, getErr
		},
		Load: func(context.Context) (*readThroughValue, error) {
			t.Fatalf("Load called after get error")
			return nil, nil
		},
	})
	if !errors.Is(err, getErr) {
		t.Fatalf("ReadThrough() error = %v, want %v", err, getErr)
	}
}

func TestReadThroughGetErrorIgnoredLoads(t *testing.T) {
	getErr := errors.New("get failed")
	got, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Get: func(context.Context) (*readThroughValue, error) {
			return nil, getErr
		},
		Load: func(context.Context) (*readThroughValue, error) {
			return &readThroughValue{token: "loaded"}, nil
		},
		IgnoreGetError: true,
	})
	if err != nil {
		t.Fatalf("ReadThrough() error = %v", err)
	}
	if got == nil || got.token != "loaded" {
		t.Fatalf("ReadThrough() token = %#v, want loaded", got)
	}
}

func TestReadThroughLoadError(t *testing.T) {
	loadErr := errors.New("load failed")
	_, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Get: func(context.Context) (*readThroughValue, error) {
			return nil, nil
		},
		Load: func(context.Context) (*readThroughValue, error) {
			return nil, loadErr
		},
	})
	if !errors.Is(err, loadErr) {
		t.Fatalf("ReadThrough() error = %v, want %v", err, loadErr)
	}
}

func TestReadThroughLoadNil(t *testing.T) {
	got, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Get: func(context.Context) (*readThroughValue, error) {
			return nil, nil
		},
		Load: func(context.Context) (*readThroughValue, error) {
			return nil, nil
		},
		Set: func(context.Context, *readThroughValue, time.Duration) error {
			t.Fatalf("Set called for nil load")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ReadThrough() error = %v", err)
	}
	if got != nil {
		t.Fatalf("ReadThrough() token = %#v, want nil", got)
	}
}

func TestReadThroughSetError(t *testing.T) {
	setErr := errors.New("set failed")
	_, err := ReadThrough(context.Background(), ReadThroughOptions[readThroughValue]{
		Get: func(context.Context) (*readThroughValue, error) {
			return nil, nil
		},
		Load: func(context.Context) (*readThroughValue, error) {
			return &readThroughValue{token: "loaded"}, nil
		},
		Set: func(context.Context, *readThroughValue, time.Duration) error {
			return setErr
		},
	})
	if !errors.Is(err, setErr) {
		t.Fatalf("ReadThrough() error = %v, want %v", err, setErr)
	}
}

func TestLockedReadThroughRequiresLock(t *testing.T) {
	_, err := LockedReadThrough(context.Background(), LockedReadThroughOptions[readThroughValue]{
		ReadThroughOptions: ReadThroughOptions[readThroughValue]{
			Get: func(context.Context) (*readThroughValue, error) {
				return nil, nil
			},
			Load: func(context.Context) (*readThroughValue, error) {
				return &readThroughValue{token: "loaded"}, nil
			},
		},
	})
	if !errors.Is(err, ErrMissingReadThroughLock) {
		t.Fatalf("LockedReadThrough() error = %v, want %v", err, ErrMissingReadThroughLock)
	}
}

func TestLockedReadThroughLockContentionRereadSuccess(t *testing.T) {
	calls := 0
	got, err := LockedReadThrough(context.Background(), LockedReadThroughOptions[readThroughValue]{
		ReadThroughOptions: ReadThroughOptions[readThroughValue]{
			Get: func(context.Context) (*readThroughValue, error) {
				calls++
				if calls == 1 {
					return nil, nil
				}
				return &readThroughValue{token: "reread"}, nil
			},
			Valid: func(value *readThroughValue) bool {
				return value.valid
			},
			Load: func(context.Context) (*readThroughValue, error) {
				t.Fatalf("Load called without lock")
				return nil, nil
			},
		},
		Lock: func(context.Context) (bool, func(), error) {
			return false, nil, nil
		},
		RereadUsable: func(value *readThroughValue) bool {
			return value.token != ""
		},
	})
	if err != nil {
		t.Fatalf("LockedReadThrough() error = %v", err)
	}
	if got == nil || got.token != "reread" {
		t.Fatalf("LockedReadThrough() token = %#v, want reread", got)
	}
}

func TestLockedReadThroughLockError(t *testing.T) {
	lockErr := errors.New("lock failed")
	_, err := LockedReadThrough(context.Background(), LockedReadThroughOptions[readThroughValue]{
		ReadThroughOptions: ReadThroughOptions[readThroughValue]{
			Get: func(context.Context) (*readThroughValue, error) {
				return nil, nil
			},
			Load: func(context.Context) (*readThroughValue, error) {
				t.Fatalf("Load called after lock error")
				return nil, nil
			},
		},
		Lock: func(context.Context) (bool, func(), error) {
			return false, nil, lockErr
		},
	})
	if !errors.Is(err, lockErr) {
		t.Fatalf("LockedReadThrough() error = %v, want %v", err, lockErr)
	}
}

func TestLockedReadThroughLockContentionRereadMiss(t *testing.T) {
	retryErr := errors.New("retry later")
	_, err := LockedReadThrough(context.Background(), LockedReadThroughOptions[readThroughValue]{
		ReadThroughOptions: ReadThroughOptions[readThroughValue]{
			Get: func(context.Context) (*readThroughValue, error) {
				return nil, nil
			},
			Load: func(context.Context) (*readThroughValue, error) {
				t.Fatalf("Load called without lock")
				return nil, nil
			},
		},
		Lock: func(context.Context) (bool, func(), error) {
			return false, nil, nil
		},
		LockMissError: retryErr,
	})
	if !errors.Is(err, retryErr) {
		t.Fatalf("LockedReadThrough() error = %v, want %v", err, retryErr)
	}
}

func TestLockedReadThroughLockContentionRereadGetErrorPropagatesByDefault(t *testing.T) {
	rereadErr := errors.New("reread failed")
	calls := 0
	_, err := LockedReadThrough(context.Background(), LockedReadThroughOptions[readThroughValue]{
		ReadThroughOptions: ReadThroughOptions[readThroughValue]{
			Get: func(context.Context) (*readThroughValue, error) {
				calls++
				if calls == 1 {
					return nil, nil
				}
				return nil, rereadErr
			},
			Load: func(context.Context) (*readThroughValue, error) {
				t.Fatalf("Load called without lock")
				return nil, nil
			},
		},
		Lock: func(context.Context) (bool, func(), error) {
			return false, nil, nil
		},
	})
	if !errors.Is(err, rereadErr) {
		t.Fatalf("LockedReadThrough() error = %v, want %v", err, rereadErr)
	}
}

func TestLockedReadThroughLockContentionRereadGetErrorIgnored(t *testing.T) {
	rereadErr := errors.New("reread failed")
	retryErr := errors.New("retry later")
	calls := 0
	_, err := LockedReadThrough(context.Background(), LockedReadThroughOptions[readThroughValue]{
		ReadThroughOptions: ReadThroughOptions[readThroughValue]{
			Get: func(context.Context) (*readThroughValue, error) {
				calls++
				if calls == 1 {
					return nil, nil
				}
				return nil, rereadErr
			},
			Load: func(context.Context) (*readThroughValue, error) {
				t.Fatalf("Load called without lock")
				return nil, nil
			},
			IgnoreGetError: true,
		},
		Lock: func(context.Context) (bool, func(), error) {
			return false, nil, nil
		},
		LockMissError: retryErr,
	})
	if !errors.Is(err, retryErr) {
		t.Fatalf("LockedReadThrough() error = %v, want %v", err, retryErr)
	}
}

func TestLockedReadThroughAcquiredLoadsSetsAndUnlocks(t *testing.T) {
	unlocked := false
	set := false
	got, err := LockedReadThrough(context.Background(), LockedReadThroughOptions[readThroughValue]{
		ReadThroughOptions: ReadThroughOptions[readThroughValue]{
			Get: func(context.Context) (*readThroughValue, error) {
				return nil, nil
			},
			Load: func(context.Context) (*readThroughValue, error) {
				return &readThroughValue{token: "loaded"}, nil
			},
			TTL: func(*readThroughValue) time.Duration {
				return time.Minute
			},
			Set: func(_ context.Context, value *readThroughValue, ttl time.Duration) error {
				set = true
				if value.token != "loaded" || ttl != time.Minute {
					t.Fatalf("Set(value, ttl) = (%#v, %v), want loaded, 1m", value, ttl)
				}
				return nil
			},
		},
		Lock: func(context.Context) (bool, func(), error) {
			return true, func() { unlocked = true }, nil
		},
	})
	if err != nil {
		t.Fatalf("LockedReadThrough() error = %v", err)
	}
	if got == nil || got.token != "loaded" {
		t.Fatalf("LockedReadThrough() token = %#v, want loaded", got)
	}
	if !set {
		t.Fatalf("Set was not called")
	}
	if !unlocked {
		t.Fatalf("unlock was not called")
	}
}
