package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type WorkerLeaseStore struct {
	db *sql.DB
}

func NewWorkerLeaseStore(db *sql.DB) *WorkerLeaseStore {
	return &WorkerLeaseStore{db: db}
}

func (s *WorkerLeaseStore) AcquireWorkerID(ctx context.Context, instanceID string, leaseTimeout time.Duration) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	leaseTimeoutSeconds := int64(leaseTimeout.Seconds())
	var workerID int64
	err = tx.QueryRowContext(
		ctx,
		`SELECT worker_id
FROM worker_id_alloc
WHERE status = 'released'
   OR (status = 'active' AND lease_time < NOW() - make_interval(secs => $1))
ORDER BY worker_id
LIMIT 1
FOR UPDATE SKIP LOCKED`,
		leaseTimeoutSeconds,
	).Scan(&workerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("no worker id available")
		}
		return 0, err
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE worker_id_alloc
SET status = 'active',
    instance_id = $2,
    lease_time = NOW()
WHERE worker_id = $1
  AND (status = 'released'
       OR (status = 'active' AND lease_time < NOW() - make_interval(secs => $3)))`,
		workerID,
		instanceID,
		leaseTimeoutSeconds,
	)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if rows != 1 {
		return 0, fmt.Errorf("failed to acquire worker id %d", workerID)
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return workerID, nil
}

func (s *WorkerLeaseStore) RenewLease(ctx context.Context, workerID int64, instanceID string) (bool, error) {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE worker_id_alloc
SET lease_time = NOW()
WHERE worker_id = $1
  AND instance_id = $2
  AND status = 'active'`,
		workerID,
		instanceID,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}

func (s *WorkerLeaseStore) ReleaseWorkerID(ctx context.Context, workerID int64, instanceID string) (bool, error) {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE worker_id_alloc
SET status = 'released',
    instance_id = ''
WHERE worker_id = $1
  AND instance_id = $2`,
		workerID,
		instanceID,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}
