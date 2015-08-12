package scheduler

// LimitIterator is a RankIterator used to limit the number of options
// that are returned before we artifically end the stream.
type LimitIterator struct {
	ctx    Context
	source RankIterator
	limit  int
	seen   int
}

// NewLimitIterator is returns a LimitIterator with a fixed limit of returned options
func NewLimitIterator(ctx Context, source RankIterator, limit int) *LimitIterator {
	iter := &LimitIterator{
		ctx:    ctx,
		source: source,
		limit:  limit,
	}
	return iter
}

func (iter *LimitIterator) Next() *RankedNode {
	if iter.seen == iter.limit {
		return nil
	}

	option := iter.source.Next()
	if option == nil {
		return nil
	}

	iter.seen += 1
	return option
}

// MaxScoreIterator is a RankIterator used to return only a single result
// of the item with the highest score. This iterator will consume all of the
// possible inputs and only returns the highest ranking result.
type MaxScoreIterator struct {
	ctx    Context
	source RankIterator
	max    *RankedNode
}

func (iter *MaxScoreIterator) Next() *RankedNode {
	for {
		option := iter.source.Next()
		if option == nil {
			break
		}

		if iter.max == nil {
			iter.max = option
			continue
		}

		if option.Score > iter.max.Score {
			iter.max = option
		}
	}
	return iter.max
}
