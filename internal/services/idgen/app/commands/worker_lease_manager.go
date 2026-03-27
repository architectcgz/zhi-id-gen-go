package commands

import (
	"context"
	"errors"
	"sync"
	"time"
)

const maxRenewFailCount = 2

type workerLeaseStore interface {
	AcquireWorkerID(ctx context.Context, instanceID string, leaseTimeout time.Duration) (int64, error)
	RenewLease(ctx context.Context, workerID int64, instanceID string) (bool, error)
	ReleaseWorkerID(ctx context.Context, workerID int64, instanceID string) (bool, error)
}

type workerLeaseManagerOptions struct {
	instanceID    string
	leaseTimeout  time.Duration
	renewInterval time.Duration
	backupCount   int
}

type WorkerLeaseManager struct {
	store workerLeaseStore
	opts  workerLeaseManagerOptions

	mu            sync.Mutex
	primarySet    bool
	primaryWorker int64
	backups       []int64
	valid         bool
	renewFail     int
	stopRenew     context.CancelFunc
}

func newWorkerLeaseManager(ctx context.Context, store workerLeaseStore, opts workerLeaseManagerOptions) (*WorkerLeaseManager, error) {
	primaryWorker, err := store.AcquireWorkerID(ctx, opts.instanceID, opts.leaseTimeout)
	if err != nil {
		return nil, err
	}

	manager := &WorkerLeaseManager{
		store:         store,
		opts:          opts,
		primarySet:    true,
		primaryWorker: primaryWorker,
		valid:         true,
	}

	for i := 0; i < opts.backupCount; i++ {
		backupWorker, err := store.AcquireWorkerID(ctx, opts.instanceID, opts.leaseTimeout)
		if err != nil {
			break
		}
		manager.backups = append(manager.backups, backupWorker)
	}

	if opts.renewInterval > 0 {
		renewCtx, cancel := context.WithCancel(context.Background())
		manager.stopRenew = cancel
		go manager.runRenewLoop(renewCtx)
	}

	return manager, nil
}

func NewDBWorkerLeaseManager(
	ctx context.Context,
	store workerLeaseStore,
	instanceID string,
	leaseTimeout time.Duration,
	renewInterval time.Duration,
	backupCount int,
) (*WorkerLeaseManager, error) {
	return newWorkerLeaseManager(ctx, store, workerLeaseManagerOptions{
		instanceID:    instanceID,
		leaseTimeout:  leaseTimeout,
		renewInterval: renewInterval,
		backupCount:   backupCount,
	})
}

func (m *WorkerLeaseManager) runRenewLoop(ctx context.Context) {
	ticker := time.NewTicker(m.opts.renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.renewOnce(context.Background())
		}
	}
}

func (m *WorkerLeaseManager) PrimaryWorkerID() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.primaryWorker
}

func (m *WorkerLeaseManager) IsWorkerIDValid() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.valid
}

func (m *WorkerLeaseManager) renewOnce(ctx context.Context) {
	m.mu.Lock()
	primaryWorker := m.primaryWorker
	backups := append([]int64(nil), m.backups...)
	instanceID := m.opts.instanceID
	m.mu.Unlock()

	ok, err := m.store.RenewLease(ctx, primaryWorker, instanceID)
	m.mu.Lock()
	if err != nil || !ok {
		m.renewFail++
		if m.renewFail >= maxRenewFailCount {
			m.valid = false
		}
	} else {
		m.renewFail = 0
	}
	m.mu.Unlock()

	if len(backups) == 0 {
		return
	}

	liveBackups := make([]int64, 0, len(backups))
	for _, backup := range backups {
		ok, err := m.store.RenewLease(ctx, backup, instanceID)
		if err == nil && ok {
			liveBackups = append(liveBackups, backup)
		}
	}

	m.mu.Lock()
	m.backups = liveBackups
	m.mu.Unlock()
}

func (m *WorkerLeaseManager) ConsumeBackupWorkerID(ctx context.Context) (int64, error) {
	m.mu.Lock()
	if len(m.backups) == 0 {
		m.mu.Unlock()
		return 0, errors.New("no backup worker id available")
	}
	backup := m.backups[0]
	oldPrimary := m.primaryWorker
	m.backups = append([]int64(nil), m.backups[1:]...)
	m.primaryWorker = backup
	m.valid = true
	m.renewFail = 0
	instanceID := m.opts.instanceID
	m.mu.Unlock()

	_, err := m.store.ReleaseWorkerID(ctx, oldPrimary, instanceID)
	if err != nil {
		return 0, err
	}
	return backup, nil
}

func (m *WorkerLeaseManager) Close(ctx context.Context) error {
	if m.stopRenew != nil {
		m.stopRenew()
	}

	m.mu.Lock()
	workers := append([]int64(nil), m.backups...)
	if m.primarySet {
		workers = append([]int64{m.primaryWorker}, workers...)
	}
	instanceID := m.opts.instanceID
	m.mu.Unlock()

	var errs []error
	for _, worker := range workers {
		if _, err := m.store.ReleaseWorkerID(ctx, worker, instanceID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
