package commands

import (
	"context"
	"testing"
	"time"
)

type stubWorkerLeaseStore struct {
	acquire func(ctx context.Context, instanceID string, leaseTimeout time.Duration) (int64, error)
	renew   func(ctx context.Context, workerID int64, instanceID string) (bool, error)
	release func(ctx context.Context, workerID int64, instanceID string) (bool, error)
}

func (s stubWorkerLeaseStore) AcquireWorkerID(ctx context.Context, instanceID string, leaseTimeout time.Duration) (int64, error) {
	return s.acquire(ctx, instanceID, leaseTimeout)
}

func (s stubWorkerLeaseStore) RenewLease(ctx context.Context, workerID int64, instanceID string) (bool, error) {
	return s.renew(ctx, workerID, instanceID)
}

func (s stubWorkerLeaseStore) ReleaseWorkerID(ctx context.Context, workerID int64, instanceID string) (bool, error) {
	return s.release(ctx, workerID, instanceID)
}

func TestWorkerLeaseManager_AcquiresPrimaryAndBackupAndPromotesBackup(t *testing.T) {
	var acquireCount int
	var released []int64

	manager, err := newWorkerLeaseManager(
		context.Background(),
		stubWorkerLeaseStore{
			acquire: func(_ context.Context, _ string, _ time.Duration) (int64, error) {
				acquireCount++
				if acquireCount == 1 {
					return 3, nil
				}
				return 4, nil
			},
			renew: func(_ context.Context, _ int64, _ string) (bool, error) {
				return true, nil
			},
			release: func(_ context.Context, workerID int64, _ string) (bool, error) {
				released = append(released, workerID)
				return true, nil
			},
		},
		workerLeaseManagerOptions{
			instanceID:    "node-a",
			leaseTimeout:  10 * time.Minute,
			renewInterval: 0,
			backupCount:   1,
		},
	)
	if err != nil {
		t.Fatalf("newWorkerLeaseManager returned error: %v", err)
	}

	if manager.PrimaryWorkerID() != 3 {
		t.Fatalf("primary worker = %d, want 3", manager.PrimaryWorkerID())
	}

	backupID, err := manager.ConsumeBackupWorkerID(context.Background())
	if err != nil {
		t.Fatalf("ConsumeBackupWorkerID returned error: %v", err)
	}
	if backupID != 4 {
		t.Fatalf("backup worker = %d, want 4", backupID)
	}
	if manager.PrimaryWorkerID() != 4 {
		t.Fatalf("primary worker after promote = %d, want 4", manager.PrimaryWorkerID())
	}
	if len(released) != 1 || released[0] != 3 {
		t.Fatalf("released = %v, want [3]", released)
	}
}

func TestWorkerLeaseManager_RenewFailureMarksPrimaryInvalid(t *testing.T) {
	manager, err := newWorkerLeaseManager(
		context.Background(),
		stubWorkerLeaseStore{
			acquire: func(_ context.Context, _ string, _ time.Duration) (int64, error) {
				return 5, nil
			},
			renew: func(_ context.Context, _ int64, _ string) (bool, error) {
				return false, nil
			},
			release: func(_ context.Context, _ int64, _ string) (bool, error) {
				return true, nil
			},
		},
		workerLeaseManagerOptions{
			instanceID:    "node-a",
			leaseTimeout:  10 * time.Minute,
			renewInterval: 0,
			backupCount:   0,
		},
	)
	if err != nil {
		t.Fatalf("newWorkerLeaseManager returned error: %v", err)
	}

	manager.renewOnce(context.Background())
	if !manager.IsWorkerIDValid() {
		t.Fatal("worker id should remain valid after first renew failure")
	}

	manager.renewOnce(context.Background())
	if manager.IsWorkerIDValid() {
		t.Fatal("worker id should be invalid after second renew failure")
	}
}
