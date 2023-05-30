package scheduler

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
)

func TestLimitIterator_EXP(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)

	nodeSizes := []int{10000, 5000, 1000, 500, 100, 50}
	allocSizes := []int{20000, 15000, 10000, 5000, 1000, 500, 200, 10}

	testCases := []limitTestCase{}

	// TODO: set lower limit based on
	// https://go.dev/play/p/kuIm5Xtk-gU

	// see stack.go before we set SetLimit from spread
	logLimit := func(n int) int {
		limit := int(math.Ceil(math.Log2(float64(n))))
		if limit < 3 {
			limit = 3
		}
		return limit
	}

	for _, ns := range nodeSizes {
		for _, as := range allocSizes {
			testCase := limitTestCase{
				name:       "existing",
				nodeCount:  ns,
				allocCount: as,
				limit:      as,
				maxSkip:    3,
			}
			if as < 100 {
				testCase.limit = 100
			}
			testCases = append(testCases, testCase)
		}
	}

	for _, ns := range nodeSizes {
		for _, as := range allocSizes {
			testCase := limitTestCase{
				name:       "limit=count/2 skip=3",
				nodeCount:  ns,
				allocCount: as,
				limit:      as / 2,
				maxSkip:    3,
			}
			if testCase.limit < 50 {
				testCase.limit = 50
			}
			testCases = append(testCases, testCase)
		}
	}

	for _, ns := range nodeSizes {
		for _, as := range allocSizes {
			testCase := limitTestCase{
				name:       "limit=count/2 skip=10",
				nodeCount:  ns,
				allocCount: as,
				limit:      as / 2,
				maxSkip:    10,
			}
			if testCase.limit < 50 {
				testCase.limit = 50
			}
			testCases = append(testCases, testCase)
		}
	}

	for _, ns := range nodeSizes {
		for _, as := range allocSizes {
			testCase := limitTestCase{
				name:       "limit=count/2 skip=count/2",
				nodeCount:  ns,
				allocCount: as,
				limit:      as / 2,
				maxSkip:    as / 2,
			}
			if testCase.limit < 50 {
				testCase.limit = 50
			}
			if testCase.maxSkip > 50 {
				testCase.maxSkip = 50
			}
			if testCase.maxSkip < 3 {
				testCase.maxSkip = 3
			}

			testCases = append(testCases, testCase)
		}
	}

	for _, ns := range nodeSizes {
		for _, as := range allocSizes {
			testCase := limitTestCase{
				name:       "limit=count/2 skip=logLimit",
				nodeCount:  ns,
				allocCount: as,
				limit:      as / 2,
				maxSkip:    logLimit(ns),
			}
			if testCase.limit < 50 {
				testCase.limit = 50
			}
			testCases = append(testCases, testCase)
		}
	}

	setupNodes := func(nodeCount int) []*RankedNode {
		nodes := []*RankedNode{}
		for i := 0; i < nodeCount; i++ {
			nodes = append(nodes, &RankedNode{Node: mock.Node(), FinalScore: 0.0})
		}
		return nodes
	}

	results := []limitTestResult{}

	testFunc := func(tc limitTestCase) func(*testing.T) {
		return func(t *testing.T) {

			now := time.Now()

			nodes := setupNodes(tc.nodeCount)
			placements := map[string]float64{}
			totalSeen := 0
			totalSkipped := 0

			for a := 0; a < tc.allocCount; a++ {

				// shuffle
				for i := range nodes {
					j := rand.Intn(i + 1)
					nodes[i], nodes[j] = nodes[j], nodes[i]
				}

				rand := NewStaticRankIterator(ctx, nodes)

				placeIter := testPlacementIterator{
					source:     rand,
					placements: placements,
				}
				limit := NewLimitIterator(ctx, placeIter, tc.limit, -1.0, tc.maxSkip)

				out := collectRanked(limit)
				selected := selectRanked(out)
				placements[selected.Node.ID] -= 1.0
				totalSeen += limit.seen
				totalSkipped += len(limit.skippedNodes)
			}

			placementCounts := map[string]int{}
			for _, score := range placements {
				placementCounts[fmt.Sprintf("score:%v", score)]++
			}

			if tc.expectCounts != nil {
				t.Logf("allocs=%d nodes=%d seen=%d skipped=%d counts=%v",
					tc.allocCount, tc.nodeCount, totalSeen, totalSkipped, placementCounts)
				must.Eq(t, tc.expectCounts, placementCounts)
			} else {

				results = append(results, limitTestResult{
					name:         tc.name,
					nodeCount:    tc.nodeCount,
					allocCount:   tc.allocCount,
					limit:        tc.limit,
					maxSkip:      tc.maxSkip,
					totalSeen:    totalSeen,
					totalSkipped: totalSkipped,
					elapsedMs:    time.Since(now).Milliseconds(),
					counts:       placementCounts,
				})
			}
		}
	}

	for _, tc := range testCases {
		t.Run(tc.caseName(), testFunc(tc))
	}

	f, _ := os.Create("/home/tim/tmp/test.csv")
	w := csv.NewWriter(f)
	headers := results[0].toHeader()
	w.Write(headers)
	for _, result := range results {
		v := result.toSlice()
		w.Write(v)
	}
	w.Flush()
}

type limitTestCase struct {
	name         string
	nodeCount    int
	allocCount   int
	limit        int
	maxSkip      int
	expectCounts map[string]int
}

func (tc limitTestCase) caseName() string {
	return fmt.Sprintf("[%s] nodes=%d allocs=%d limit=%d maxSkip=%d",
		tc.name, tc.nodeCount, tc.allocCount, tc.limit, tc.maxSkip)
}

type limitTestResult struct {
	name         string
	nodeCount    int
	allocCount   int
	limit        int
	maxSkip      int
	totalSeen    int
	totalSkipped int
	elapsedMs    int64
	counts       map[string]int
}

func (tc limitTestResult) toSlice() []string {
	return []string{
		tc.name,
		fmt.Sprintf("%d", tc.nodeCount),
		fmt.Sprintf("%d", tc.allocCount),
		fmt.Sprintf("%d", tc.limit),
		fmt.Sprintf("%d", tc.maxSkip),
		fmt.Sprintf("%d", tc.totalSeen),
		fmt.Sprintf("%d", tc.totalSkipped),
		fmt.Sprintf("%d", tc.elapsedMs),
		fmt.Sprintf("%v", tc.counts),
	}
}

func (tc limitTestResult) toHeader() []string {
	return []string{
		"testName",
		"nodeCount",
		"allocCount",
		"limit",
		"maxSkip",
		"totalSeen",
		"totalSkipped",
		"elapsedMs",
		"counts",
	}
}

type testPlacementIterator struct {
	source     RankIterator
	placements map[string]float64
}

func (iter testPlacementIterator) Next() *RankedNode {
	option := iter.source.Next()
	if option == nil {
		return nil
	}
	option.FinalScore = iter.placements[option.Node.ID]
	return option
}

func (iter testPlacementIterator) Reset() {}

func selectRanked(nodes []*RankedNode) *RankedNode {
	var bestNode *RankedNode

	for _, node := range nodes {
		if bestNode == nil {
			bestNode = node
			continue
		}
		if node.FinalScore > bestNode.FinalScore {
			bestNode = node
		}
	}
	return bestNode
}
