package domain

type SegmentAllocation struct {
	BizTag string
	MaxID  int64
	Step   int
}

func (a SegmentAllocation) StartID() int64 {
	return a.MaxID - int64(a.Step)
}
