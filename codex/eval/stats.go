//go:build fts5

package eval

import (
	"math"
	"math/rand"
	"sort"
)

// =============================================================================
// Statistical Analysis Module
//
// Implements the statistical tests specified in the evaluation framework doc
// (lines 2032-2041):
//   - Mann-Whitney U test: between-condition comparisons
//   - Bootstrap confidence intervals: 10K resamples
//   - Wilcoxon signed-rank test: paired comparisons (Experiment 3B)
//   - Spearman rank correlation: trend analysis (Experiment 3C)
// =============================================================================

// MannWhitneyResult holds the result of a Mann-Whitney U test.
type MannWhitneyResult struct {
	U1         float64 `json:"u1"`          // U statistic for sample 1
	U2         float64 `json:"u2"`          // U statistic for sample 2
	Z          float64 `json:"z"`           // z-approximation for large samples
	P          float64 `json:"p"`           // two-tailed p-value (approximate)
	Significant bool   `json:"significant"` // p < 0.05
	EffectSize float64 `json:"effect_size"` // r = Z / sqrt(N)
}

// MannWhitneyU performs the Mann-Whitney U test (Wilcoxon rank-sum test)
// for comparing two independent samples.
// This is the primary test for between-condition comparisons in Experiment 3A.
func MannWhitneyU(sample1, sample2 []float64) MannWhitneyResult {
	n1, n2 := len(sample1), len(sample2)
	if n1 == 0 || n2 == 0 {
		return MannWhitneyResult{}
	}

	// Combine all values and track which group each belongs to
	groups := make([]int, 0, n1+n2)
	combined := make([]float64, 0, n1+n2)
	for _, v := range sample1 {
		combined = append(combined, v)
		groups = append(groups, 0)
	}
	for _, v := range sample2 {
		combined = append(combined, v)
		groups = append(groups, 1)
	}

	// Sort by value, preserving group membership
	indices := make([]int, len(combined))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return combined[indices[i]] < combined[indices[j]]
	})

	// Assign ranks with tie handling (average rank)
	ranks := make([]float64, len(combined))
	for i := 0; i < len(indices); {
		j := i + 1
		for j < len(indices) && combined[indices[j]] == combined[indices[i]] {
			j++
		}
		avgRank := float64(i+j+1) / 2.0
		for k := i; k < j; k++ {
			ranks[indices[k]] = avgRank
		}
		i = j
	}

	// Sum ranks for each group
	var rankSum1, rankSum2 float64
	for i, g := range groups {
		if g == 0 {
			rankSum1 += ranks[i]
		} else {
			rankSum2 += ranks[i]
		}
	}

	// Calculate U statistics
	fn1, fn2 := float64(n1), float64(n2)
	u1 := rankSum1 - fn1*(fn1+1)/2
	u2 := rankSum2 - fn2*(fn2+1)/2

	// Normal approximation for z-score
	mu := fn1 * fn2 / 2
	sigma := math.Sqrt(fn1 * fn2 * (fn1 + fn2 + 1) / 12)

	var z float64
	if sigma > 0 {
		z = (math.Min(u1, u2) - mu) / sigma
	}

	// Two-tailed p-value from normal approximation
	p := 2 * normalCDF(-math.Abs(z))

	// Effect size r = Z / sqrt(N)
	n := float64(n1 + n2)
	effectSize := math.Abs(z) / math.Sqrt(n)

	return MannWhitneyResult{
		U1:          u1,
		U2:          u2,
		Z:           z,
		P:           p,
		Significant: p < 0.05,
		EffectSize:  effectSize,
	}
}

// BootstrapCI computes a bootstrap confidence interval for the mean of a sample.
// Uses 10,000 resamples as specified in the evaluation framework.
type BootstrapCI struct {
	Mean    float64 `json:"mean"`
	Lower   float64 `json:"lower"`   // lower bound of CI
	Upper   float64 `json:"upper"`   // upper bound of CI
	Level   float64 `json:"level"`   // confidence level (e.g., 0.95)
	Samples int     `json:"samples"` // number of bootstrap resamples
}

// Bootstrap computes a bootstrap confidence interval for the mean.
// level should be 0.95 for a 95% CI. resamples defaults to 10000 if <= 0.
func Bootstrap(data []float64, level float64, resamples int) BootstrapCI {
	if len(data) == 0 {
		return BootstrapCI{Level: level}
	}
	if resamples <= 0 {
		resamples = 10000
	}
	if level <= 0 || level >= 1 {
		level = 0.95
	}

	n := len(data)
	rng := rand.New(rand.NewSource(42)) // deterministic for reproducibility

	means := make([]float64, resamples)
	for i := 0; i < resamples; i++ {
		var sum float64
		for j := 0; j < n; j++ {
			sum += data[rng.Intn(n)]
		}
		means[i] = sum / float64(n)
	}

	sort.Float64s(means)

	alpha := 1 - level
	lowerIdx := int(math.Floor(alpha / 2 * float64(resamples)))
	upperIdx := int(math.Ceil((1 - alpha/2) * float64(resamples)))
	if lowerIdx < 0 {
		lowerIdx = 0
	}
	if upperIdx >= resamples {
		upperIdx = resamples - 1
	}

	return BootstrapCI{
		Mean:    mean(data),
		Lower:   means[lowerIdx],
		Upper:   means[upperIdx],
		Level:   level,
		Samples: resamples,
	}
}

// WilcoxonResult holds the result of a Wilcoxon signed-rank test.
type WilcoxonResult struct {
	W          float64 `json:"w"`           // test statistic (smaller of W+ and W-)
	Z          float64 `json:"z"`           // z-approximation
	P          float64 `json:"p"`           // two-tailed p-value
	Significant bool   `json:"significant"` // p < 0.05
	NPairs     int     `json:"n_pairs"`     // number of non-zero differences
}

// WilcoxonSignedRank performs the Wilcoxon signed-rank test for paired samples.
// Used in Experiment 3B to compare task pairs with shared pitfalls.
func WilcoxonSignedRank(sample1, sample2 []float64) WilcoxonResult {
	n := len(sample1)
	if n == 0 || n != len(sample2) {
		return WilcoxonResult{}
	}

	// Compute differences and filter zeros
	type diff struct {
		absDiff float64
		sign    float64 // +1 or -1
		rank    float64
	}

	var diffs []diff
	for i := 0; i < n; i++ {
		d := sample1[i] - sample2[i]
		if d == 0 {
			continue
		}
		sign := 1.0
		if d < 0 {
			sign = -1.0
		}
		diffs = append(diffs, diff{absDiff: math.Abs(d), sign: sign})
	}

	if len(diffs) == 0 {
		return WilcoxonResult{NPairs: 0, P: 1.0}
	}

	// Sort by absolute difference
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].absDiff < diffs[j].absDiff
	})

	// Assign ranks with tie handling
	for i := 0; i < len(diffs); {
		j := i + 1
		for j < len(diffs) && diffs[j].absDiff == diffs[i].absDiff {
			j++
		}
		avgRank := float64(i+j+1) / 2.0
		for k := i; k < j; k++ {
			diffs[k].rank = avgRank
		}
		i = j
	}

	// Sum signed ranks
	var wPlus, wMinus float64
	for _, d := range diffs {
		if d.sign > 0 {
			wPlus += d.rank
		} else {
			wMinus += d.rank
		}
	}

	w := math.Min(wPlus, wMinus)
	nf := float64(len(diffs))

	// Normal approximation
	mu := nf * (nf + 1) / 4
	sigma := math.Sqrt(nf * (nf + 1) * (2*nf + 1) / 24)

	var z float64
	if sigma > 0 {
		z = (w - mu) / sigma
	}

	p := 2 * normalCDF(-math.Abs(z))

	return WilcoxonResult{
		W:           w,
		Z:           z,
		P:           p,
		Significant: p < 0.05,
		NPairs:      len(diffs),
	}
}

// SpearmanResult holds the result of Spearman's rank correlation.
type SpearmanResult struct {
	Rho        float64 `json:"rho"`         // Spearman correlation coefficient
	P          float64 `json:"p"`           // approximate p-value
	Significant bool   `json:"significant"` // p < 0.05
	N          int     `json:"n"`
}

// SpearmanCorrelation computes Spearman's rank correlation coefficient.
// Used in Experiment 3C to detect quality trends across sessions.
func SpearmanCorrelation(x, y []float64) SpearmanResult {
	n := len(x)
	if n < 3 || n != len(y) {
		return SpearmanResult{N: n}
	}

	// Rank both variables
	rankX := computeRanks(x)
	rankY := computeRanks(y)

	// Compute Pearson correlation on ranks
	var sumD2 float64
	for i := 0; i < n; i++ {
		d := rankX[i] - rankY[i]
		sumD2 += d * d
	}

	nf := float64(n)
	rho := 1 - (6*sumD2)/(nf*(nf*nf-1))

	// Approximate p-value using t-distribution approximation
	t := rho * math.Sqrt((nf-2)/(1-rho*rho))
	// Two-tailed p-value from t-distribution (approximate using normal for n > 10)
	p := 2 * normalCDF(-math.Abs(t))
	if n <= 10 {
		// For small samples, use a rougher approximation
		p = 2 * normalCDF(-math.Abs(t) * math.Sqrt(nf-2) / math.Sqrt(nf))
	}

	return SpearmanResult{
		Rho:         rho,
		P:           p,
		Significant: p < 0.05,
		N:           n,
	}
}

// ConditionComparison holds a statistical comparison between two conditions.
type ConditionComparison struct {
	Condition1   string            `json:"condition_1"`
	Condition2   string            `json:"condition_2"`
	Metric       string            `json:"metric"`
	MannWhitney  MannWhitneyResult `json:"mann_whitney"`
	Bootstrap1   BootstrapCI       `json:"bootstrap_1"`
	Bootstrap2   BootstrapCI       `json:"bootstrap_2"`
	MeanDiff     float64           `json:"mean_diff"`     // condition1 - condition2
	MeanDiffPct  float64           `json:"mean_diff_pct"` // percentage improvement
}

// CompareConditions performs a statistical comparison of two conditions on a given metric.
func CompareConditions(name1 string, vals1 []float64, name2 string, vals2 []float64, metric string) ConditionComparison {
	mw := MannWhitneyU(vals1, vals2)
	b1 := Bootstrap(vals1, 0.95, 10000)
	b2 := Bootstrap(vals2, 0.95, 10000)

	diff := b1.Mean - b2.Mean
	var diffPct float64
	if b2.Mean != 0 {
		diffPct = (diff / math.Abs(b2.Mean)) * 100
	}

	return ConditionComparison{
		Condition1:  name1,
		Condition2:  name2,
		Metric:      metric,
		MannWhitney: mw,
		Bootstrap1:  b1,
		Bootstrap2:  b2,
		MeanDiff:    diff,
		MeanDiffPct: diffPct,
	}
}

// =============================================================================
// Helper functions
// =============================================================================

// normalCDF computes the cumulative distribution function of the standard normal.
func normalCDF(z float64) float64 {
	return 0.5 * math.Erfc(-z/math.Sqrt(2))
}

// mean computes the arithmetic mean of a slice.
func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// computeRanks assigns ranks to values (with average for ties).
func computeRanks(values []float64) []float64 {
	n := len(values)
	type indexedValue struct {
		val float64
		idx int
	}

	indexed := make([]indexedValue, n)
	for i, v := range values {
		indexed[i] = indexedValue{val: v, idx: i}
	}
	sort.Slice(indexed, func(i, j int) bool {
		return indexed[i].val < indexed[j].val
	})

	ranks := make([]float64, n)
	for i := 0; i < n; {
		j := i + 1
		for j < n && indexed[j].val == indexed[i].val {
			j++
		}
		avgRank := float64(i+j+1) / 2.0
		for k := i; k < j; k++ {
			ranks[indexed[k].idx] = avgRank
		}
		i = j
	}
	return ranks
}
