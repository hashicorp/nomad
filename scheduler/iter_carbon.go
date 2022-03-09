package scheduler

import (
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

type NoopIterator struct {
	source RankIterator
}

func (iter *NoopIterator) Next() *RankedNode {
	return iter.source.Next()
}

func (iter *NoopIterator) Reset() {
	iter.source.Reset()
}

type CarbonScoreIterator struct {
	def    float64
	ctx    Context
	source RankIterator
	logger hclog.Logger
}

func NewCarbonScoreIterator(ctx Context, source RankIterator, schedConfig *structs.SchedulerConfiguration) RankIterator {
	logger := ctx.Logger().Named("carbon")

	// Disable carbon scoring
	if schedConfig.ScoringWeights["carbon"] == 0 {
		logger.Info("Carbon scoring disabled; please set scoring_weights.carbon > 0 to enable")

		return &NoopIterator{source: source}
	}

	return &CarbonScoreIterator{
		def:    schedConfig.CarbonDefaultScore,
		ctx:    ctx,
		source: source,
		logger: logger,
	}
}

// Next yields a ranked option or nil if exhausted
func (c *CarbonScoreIterator) Next() *RankedNode {
	option := c.source.Next()
	if option == nil {
		return nil
	}

	score := c.def
	strScore := option.Node.Attributes["energy.carbon_score"]
	if strScore != "" {
		var err error
		score, err = strconv.ParseFloat(strScore, 64)
		if err != nil {
			//TODO(carbon) don't log every time we hit an invalid node
			c.logger.Error("invalid carbon score; must be a float", "raw", strScore, "error", err)
			score = c.def
		}
	}

	// Normalize score
	const maxScore = 100.0
	score /= maxScore

	// More carbon == worse score
	score *= -1.0

	// Enforce bounds
	if score < -1.0 {
		score = -1.0
	} else if score >= -0 {
		score = 0
	}

	option.Scores["carbon"] = score
	c.ctx.Metrics().ScoreNode(option.Node, "carbon", score)
	return option
}

// Reset is invoked when an allocation has been placed
// to reset any stale state.
func (c *CarbonScoreIterator) Reset() {
	c.source.Reset()
}
