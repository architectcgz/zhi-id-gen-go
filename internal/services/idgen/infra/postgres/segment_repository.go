package postgres

import (
	"context"
	"database/sql"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type SegmentRepository struct {
	db *sql.DB
}

func NewSegmentRepository(db *sql.DB) *SegmentRepository {
	return &SegmentRepository{db: db}
}

func (r *SegmentRepository) LoadSegmentRange(ctx context.Context, bizTag string) (domain.SegmentAllocation, error) {
	var allocation domain.SegmentAllocation
	err := r.db.QueryRowContext(
		ctx,
		`UPDATE leaf_alloc
SET max_id = max_id + step,
    update_time = CURRENT_TIMESTAMP,
    version = version + 1
WHERE biz_tag = $1
RETURNING biz_tag, max_id, step`,
		bizTag,
	).Scan(&allocation.BizTag, &allocation.MaxID, &allocation.Step)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.SegmentAllocation{}, domain.NewBizTagNotExists(bizTag)
		}
		return domain.SegmentAllocation{}, err
	}
	return allocation, nil
}

func (r *SegmentRepository) ListBizTags(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT biz_tag FROM leaf_alloc ORDER BY biz_tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var bizTag string
		if err := rows.Scan(&bizTag); err != nil {
			return nil, err
		}
		tags = append(tags, bizTag)
	}

	return tags, rows.Err()
}

func (r *SegmentRepository) IsInitialized() bool {
	return r.db != nil
}
