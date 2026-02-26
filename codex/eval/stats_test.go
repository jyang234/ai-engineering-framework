//go:build fts5

package eval

import (
	"math"
	"testing"
)

// =============================================================================
// Statistical Analysis Tests — Given-When-Then
// =============================================================================

// --- Mann-Whitney U Test ---

func TestMannWhitneyU_GivenIdenticalSamples_ThenNotSignificant(t *testing.T) {
	// Given: two identical samples
	s1 := []float64{5, 5, 5, 5, 5}
	s2 := []float64{5, 5, 5, 5, 5}

	// When: we run the Mann-Whitney U test
	result := MannWhitneyU(s1, s2)

	// Then: p-value should be 1.0 (not significant)
	if result.Significant {
		t.Error("identical samples should not be significant")
	}
}

func TestMannWhitneyU_GivenClearlyDifferentSamples_ThenSignificant(t *testing.T) {
	// Given: two clearly separated samples
	s1 := []float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	s2 := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// When: we run the Mann-Whitney U test
	result := MannWhitneyU(s1, s2)

	// Then: should be significant (p < 0.05) with medium-large effect
	if !result.Significant {
		t.Errorf("clearly different samples should be significant, p=%f", result.P)
	}
	if result.EffectSize < 0.3 {
		t.Errorf("effect size = %f, want >= 0.3 for clearly different samples", result.EffectSize)
	}
}

func TestMannWhitneyU_GivenEmptySample1_ThenZeroResult(t *testing.T) {
	// Given: empty sample 1
	s1 := []float64{}
	s2 := []float64{1, 2, 3}

	// When: we run the test
	result := MannWhitneyU(s1, s2)

	// Then: result should be zero-valued
	if result.U1 != 0 || result.U2 != 0 {
		t.Errorf("expected zero U stats for empty sample, got U1=%f U2=%f", result.U1, result.U2)
	}
}

func TestMannWhitneyU_GivenEmptySample2_ThenZeroResult(t *testing.T) {
	// Given: empty sample 2
	s1 := []float64{1, 2, 3}
	s2 := []float64{}

	// When: we run the test
	result := MannWhitneyU(s1, s2)

	// Then: zero-valued
	if result.U1 != 0 {
		t.Errorf("U1 = %f, want 0", result.U1)
	}
}

func TestMannWhitneyU_GivenUStatisticsSum_ThenEqualsN1TimesN2(t *testing.T) {
	// Given: two samples of known sizes
	s1 := []float64{3, 6, 9, 12}
	s2 := []float64{1, 4, 7, 10, 13}

	// When: we run the test
	result := MannWhitneyU(s1, s2)

	// Then: U1 + U2 = n1 * n2 (mathematical property of Mann-Whitney)
	n1n2 := float64(len(s1)) * float64(len(s2))
	if math.Abs(result.U1+result.U2-n1n2) > 0.01 {
		t.Errorf("U1(%f) + U2(%f) = %f, want %f", result.U1, result.U2, result.U1+result.U2, n1n2)
	}
}

func TestMannWhitneyU_GivenTiedValues_ThenAverageRanksUsed(t *testing.T) {
	// Given: samples with tied values
	s1 := []float64{1, 2, 3}
	s2 := []float64{2, 3, 4}

	// When: we run the test
	result := MannWhitneyU(s1, s2)

	// Then: should produce a valid result without errors
	if math.IsNaN(result.P) {
		t.Error("p-value should not be NaN with tied values")
	}
}

func TestMannWhitneyU_GivenSingleElementSamples_ThenValidResult(t *testing.T) {
	// Given: single-element samples
	s1 := []float64{10}
	s2 := []float64{1}

	// When: we run the test
	result := MannWhitneyU(s1, s2)

	// Then: U statistics should be valid
	if result.U1+result.U2 != 1.0 {
		t.Errorf("U1+U2 = %f, want 1.0 for single-element samples", result.U1+result.U2)
	}
}

// --- Bootstrap Confidence Intervals ---

func TestBootstrap_GivenEmptyData_ThenZeroMean(t *testing.T) {
	// Given: empty data
	// When: we compute bootstrap CI
	result := Bootstrap(nil, 0.95, 1000)

	// Then: mean is 0
	if result.Mean != 0 {
		t.Errorf("mean = %f, want 0 for empty data", result.Mean)
	}
}

func TestBootstrap_GivenSingleValue_ThenCIEqualsValue(t *testing.T) {
	// Given: a single value
	data := []float64{42.0}

	// When: we compute bootstrap CI
	result := Bootstrap(data, 0.95, 1000)

	// Then: mean, lower, and upper should all be 42
	if result.Mean != 42.0 {
		t.Errorf("mean = %f, want 42.0", result.Mean)
	}
	if result.Lower != 42.0 || result.Upper != 42.0 {
		t.Errorf("CI should be [42, 42] for single value, got [%f, %f]", result.Lower, result.Upper)
	}
}

func TestBootstrap_GivenNormalishData_ThenCIContainsMean(t *testing.T) {
	// Given: data drawn from a roughly normal distribution
	data := []float64{8.0, 8.5, 7.5, 9.0, 7.0, 8.2, 8.8, 7.3, 9.1, 8.1}

	// When: we compute 95% CI
	result := Bootstrap(data, 0.95, 10000)

	// Then: mean should be within the CI
	if result.Mean < result.Lower || result.Mean > result.Upper {
		t.Errorf("mean %f should be within CI [%f, %f]", result.Mean, result.Lower, result.Upper)
	}
}

func TestBootstrap_GivenHighConfidence_ThenWiderCI(t *testing.T) {
	// Given: the same data
	data := []float64{5, 6, 7, 8, 9, 10, 11, 12, 13, 14}

	// When: we compute 90% and 99% CIs
	ci90 := Bootstrap(data, 0.90, 10000)
	ci99 := Bootstrap(data, 0.99, 10000)

	// Then: 99% CI should be wider than 90% CI
	width90 := ci90.Upper - ci90.Lower
	width99 := ci99.Upper - ci99.Lower
	if width99 <= width90 {
		t.Errorf("99%% CI width (%f) should be > 90%% CI width (%f)", width99, width90)
	}
}

func TestBootstrap_GivenDefaultResamples_ThenUsesTenThousand(t *testing.T) {
	// Given: resamples = 0 (should use default)
	data := []float64{1, 2, 3, 4, 5}

	// When: we compute with 0 resamples
	result := Bootstrap(data, 0.95, 0)

	// Then: samples field should be 10000 (default)
	if result.Samples != 10000 {
		t.Errorf("samples = %d, want 10000 (default)", result.Samples)
	}
}

func TestBootstrap_GivenInvalidLevel_ThenDefaultsToNinetyFive(t *testing.T) {
	// Given: invalid confidence level
	data := []float64{1, 2, 3}

	// When: level > 1
	result := Bootstrap(data, 1.5, 100)

	// Then: defaults to 0.95
	if result.Level != 0.95 {
		t.Errorf("level = %f, want 0.95 (default)", result.Level)
	}
}

func TestBootstrap_GivenDeterministicSeed_ThenReproducible(t *testing.T) {
	// Given: the same data
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// When: we compute twice
	r1 := Bootstrap(data, 0.95, 1000)
	r2 := Bootstrap(data, 0.95, 1000)

	// Then: results should be identical (deterministic seed=42)
	if r1.Lower != r2.Lower || r1.Upper != r2.Upper {
		t.Errorf("non-reproducible: [%f,%f] vs [%f,%f]", r1.Lower, r1.Upper, r2.Lower, r2.Upper)
	}
}

// --- Wilcoxon Signed-Rank Test ---

func TestWilcoxonSignedRank_GivenIdenticalPairs_ThenNotSignificant(t *testing.T) {
	// Given: paired samples with zero differences
	s1 := []float64{5, 5, 5, 5, 5}
	s2 := []float64{5, 5, 5, 5, 5}

	// When: we run the Wilcoxon signed-rank test
	result := WilcoxonSignedRank(s1, s2)

	// Then: p = 1.0 (all differences are zero, so NPairs = 0)
	if result.NPairs != 0 {
		t.Errorf("NPairs = %d, want 0 (all diffs are zero)", result.NPairs)
	}
	if result.P != 1.0 {
		t.Errorf("P = %f, want 1.0 for identical pairs", result.P)
	}
}

func TestWilcoxonSignedRank_GivenConsistentImprovements_ThenSignificant(t *testing.T) {
	// Given: sample1 consistently better than sample2
	s1 := []float64{80, 85, 90, 88, 92, 87, 91, 86, 89, 93}
	s2 := []float64{70, 72, 75, 73, 78, 71, 76, 74, 77, 79}

	// When: we run the test
	result := WilcoxonSignedRank(s1, s2)

	// Then: should be significant
	if !result.Significant {
		t.Errorf("consistent improvements should be significant, p=%f", result.P)
	}
	if result.NPairs != 10 {
		t.Errorf("NPairs = %d, want 10", result.NPairs)
	}
}

func TestWilcoxonSignedRank_GivenEmptySamples_ThenZeroResult(t *testing.T) {
	// Given: empty samples
	result := WilcoxonSignedRank(nil, nil)

	// Then: zero result
	if result.NPairs != 0 {
		t.Errorf("NPairs = %d, want 0", result.NPairs)
	}
}

func TestWilcoxonSignedRank_GivenDifferentLengths_ThenZeroResult(t *testing.T) {
	// Given: samples of different lengths
	s1 := []float64{1, 2, 3}
	s2 := []float64{1, 2}

	// When: we run the test
	result := WilcoxonSignedRank(s1, s2)

	// Then: zero result (invalid input)
	if result.NPairs != 0 {
		t.Errorf("NPairs should be 0 for mismatched lengths")
	}
}

func TestWilcoxonSignedRank_GivenMixedDirections_ThenLessSignificant(t *testing.T) {
	// Given: paired samples with mixed improvement directions
	s1 := []float64{10, 5, 10, 5, 10}
	s2 := []float64{5, 10, 5, 10, 5}

	// When: we run the test
	result := WilcoxonSignedRank(s1, s2)

	// Then: should not be significant (mixed directions cancel out)
	if result.Significant {
		t.Errorf("mixed directions should not be significant, p=%f", result.P)
	}
}

// --- Spearman Correlation ---

func TestSpearmanCorrelation_GivenPerfectPositiveCorrelation_ThenRhoIsOne(t *testing.T) {
	// Given: perfectly correlated data
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	y := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	// When: we compute Spearman correlation
	result := SpearmanCorrelation(x, y)

	// Then: rho should be 1.0
	if math.Abs(result.Rho-1.0) > 0.001 {
		t.Errorf("rho = %f, want 1.0 for perfect positive correlation", result.Rho)
	}
}

func TestSpearmanCorrelation_GivenPerfectNegativeCorrelation_ThenRhoIsMinusOne(t *testing.T) {
	// Given: perfectly negatively correlated data
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	y := []float64{100, 90, 80, 70, 60, 50, 40, 30, 20, 10}

	// When: we compute Spearman correlation
	result := SpearmanCorrelation(x, y)

	// Then: rho should be -1.0
	if math.Abs(result.Rho+1.0) > 0.001 {
		t.Errorf("rho = %f, want -1.0 for perfect negative correlation", result.Rho)
	}
}

func TestSpearmanCorrelation_GivenNoCorrelation_ThenRhoNearZero(t *testing.T) {
	// Given: uncorrelated data
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	y := []float64{5, 3, 8, 1, 10, 2, 7, 4, 6, 9} // shuffled

	// When: we compute Spearman correlation
	result := SpearmanCorrelation(x, y)

	// Then: rho should be near zero (within ±0.5 for shuffled data)
	if math.Abs(result.Rho) > 0.5 {
		t.Errorf("rho = %f, expected near zero for uncorrelated data", result.Rho)
	}
}

func TestSpearmanCorrelation_GivenTooFewPoints_ThenEmptyResult(t *testing.T) {
	// Given: fewer than 3 data points
	x := []float64{1, 2}
	y := []float64{3, 4}

	// When: we compute correlation
	result := SpearmanCorrelation(x, y)

	// Then: N=2, Rho=0 (insufficient data)
	if result.N != 2 {
		t.Errorf("N = %d, want 2", result.N)
	}
	if result.Rho != 0 {
		t.Errorf("Rho = %f, want 0 for insufficient data", result.Rho)
	}
}

func TestSpearmanCorrelation_GivenDifferentLengths_ThenEmptyResult(t *testing.T) {
	// Given: x and y of different lengths
	x := []float64{1, 2, 3}
	y := []float64{4, 5}

	// When: we compute correlation
	result := SpearmanCorrelation(x, y)

	// Then: N reflects actual length, Rho=0
	if result.Rho != 0 {
		t.Errorf("Rho = %f, want 0 for mismatched lengths", result.Rho)
	}
}

func TestSpearmanCorrelation_GivenMonotonicSessionData_ThenPositiveRhoAndSignificant(t *testing.T) {
	// Given: quality scores that improve across sessions (Experiment 3C scenario)
	sessions := []float64{1, 2, 3, 4, 5}
	quality := []float64{6.0, 7.2, 7.5, 8.1, 8.8}

	// When: we compute Spearman correlation
	result := SpearmanCorrelation(sessions, quality)

	// Then: strong positive correlation
	if result.Rho < 0.9 {
		t.Errorf("rho = %f, want >= 0.9 for monotonic improvement", result.Rho)
	}
}

// --- CompareConditions ---

func TestCompareConditions_GivenTwoConditions_ThenAllFieldsPopulated(t *testing.T) {
	// Given: two conditions with sample data
	vals1 := []float64{8.0, 8.5, 7.5, 9.0, 7.0}
	vals2 := []float64{5.0, 5.5, 4.5, 6.0, 4.0}

	// When: we compare them
	comp := CompareConditions("aef-full", vals1, "baseline", vals2, "judge_combined")

	// Then: all fields populated
	if comp.Condition1 != "aef-full" {
		t.Errorf("Condition1 = %q", comp.Condition1)
	}
	if comp.Condition2 != "baseline" {
		t.Errorf("Condition2 = %q", comp.Condition2)
	}
	if comp.Metric != "judge_combined" {
		t.Errorf("Metric = %q", comp.Metric)
	}
	if comp.MeanDiff <= 0 {
		t.Errorf("MeanDiff = %f, expected positive (aef-full > baseline)", comp.MeanDiff)
	}
	if comp.MeanDiffPct <= 0 {
		t.Errorf("MeanDiffPct = %f, expected positive", comp.MeanDiffPct)
	}
	if comp.Bootstrap1.Mean == 0 {
		t.Error("Bootstrap1 mean should not be zero")
	}
	if comp.Bootstrap2.Mean == 0 {
		t.Error("Bootstrap2 mean should not be zero")
	}
}

func TestCompareConditions_GivenSignificantDifference_ThenMannWhitneySignificant(t *testing.T) {
	// Given: two conditions with clearly different scores
	vals1 := []float64{15, 16, 17, 18, 19, 20, 21, 22, 23, 24}
	vals2 := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// When: we compare them
	comp := CompareConditions("high", vals1, "low", vals2, "score")

	// Then: Mann-Whitney should be significant
	if !comp.MannWhitney.Significant {
		t.Errorf("clearly different conditions should be significant, p=%f", comp.MannWhitney.P)
	}
}

// --- normalCDF ---

func TestNormalCDF_GivenZeroZ_ThenHalf(t *testing.T) {
	// Given: z = 0
	result := normalCDF(0)

	// Then: CDF = 0.5
	if math.Abs(result-0.5) > 0.001 {
		t.Errorf("normalCDF(0) = %f, want 0.5", result)
	}
}

func TestNormalCDF_GivenLargePositiveZ_ThenNearOne(t *testing.T) {
	// Given: z = 3 (very significant)
	result := normalCDF(3.0)

	// Then: CDF ≈ 0.9987
	if result < 0.99 {
		t.Errorf("normalCDF(3) = %f, want > 0.99", result)
	}
}

func TestNormalCDF_GivenLargeNegativeZ_ThenNearZero(t *testing.T) {
	// Given: z = -3
	result := normalCDF(-3.0)

	// Then: CDF ≈ 0.0013
	if result > 0.01 {
		t.Errorf("normalCDF(-3) = %f, want < 0.01", result)
	}
}

// --- mean ---

func TestMean_GivenEmptySlice_ThenZero(t *testing.T) {
	if mean(nil) != 0 {
		t.Error("mean of nil should be 0")
	}
}

func TestMean_GivenValues_ThenCorrectMean(t *testing.T) {
	result := mean([]float64{2, 4, 6, 8, 10})
	if result != 6.0 {
		t.Errorf("mean = %f, want 6.0", result)
	}
}

// --- computeRanks ---

func TestComputeRanks_GivenDistinctValues_ThenSequentialRanks(t *testing.T) {
	// Given: distinct values
	values := []float64{30, 10, 20}

	// When: we compute ranks
	ranks := computeRanks(values)

	// Then: ranks are 3, 1, 2 (based on sorted position)
	expected := []float64{3, 1, 2}
	for i, r := range ranks {
		if r != expected[i] {
			t.Errorf("rank[%d] = %f, want %f", i, r, expected[i])
		}
	}
}

func TestComputeRanks_GivenTiedValues_ThenAverageRanks(t *testing.T) {
	// Given: values with ties
	values := []float64{10, 20, 20, 30}

	// When: we compute ranks
	ranks := computeRanks(values)

	// Then: tied values get average rank (2+3)/2 = 2.5
	if ranks[0] != 1.0 {
		t.Errorf("rank[0] = %f, want 1.0", ranks[0])
	}
	if ranks[1] != 2.5 || ranks[2] != 2.5 {
		t.Errorf("tied ranks = %f, %f, want 2.5, 2.5", ranks[1], ranks[2])
	}
	if ranks[3] != 4.0 {
		t.Errorf("rank[3] = %f, want 4.0", ranks[3])
	}
}
