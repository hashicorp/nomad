package scheduler

import (
	"strconv"

	"github.com/hashicorp/go-hclog"
)

type CarbonScoreIterator struct {
	ctx    Context
	source RankIterator
	logger hclog.Logger
}

func NewCarbonScoreIterator(ctx Context, source RankIterator) *CarbonScoreIterator {
	return &CarbonScoreIterator{
		ctx:    ctx,
		source: source,
		logger: ctx.Logger().Named("carbon"),
	}
}

// Next yields a ranked option or nil if exhausted
func (c *CarbonScoreIterator) Next() *RankedNode {
	option := c.source.Next()
	if option == nil {
		return nil
	}

	strScore := option.Node.Meta["carbon_score"]
	if strScore == "" {
		//TODO(carbon) No carbon, set default?
		return option
	}

	score, err := strconv.ParseFloat(strScore, 64)
	if err != nil {
		//TODO(carbon) don't log every time we hit an invalid node
		c.logger.Error("invalid carbon score; must be a float", "raw", strScore, "error", err)
		return option
	}

	//TODO(carbon) Normalize score
	// More carbon == worse score
	score *= -1.0

	option.Scores = append(option.Scores, score)
	c.ctx.Metrics().ScoreNode(option.Node, "carbon", score)
	return option
}

// Reset is invoked when an allocation has been placed
// to reset any stale state.
func (c *CarbonScoreIterator) Reset() {
	c.source.Reset()
}
