package similarity

import "math"

// BLEU4 computes BLEU-4 (modified n-gram precision for n=1..4 with the standard
// brevity penalty) and scales the result to 0-100. Smoothing: when an n-gram
// order has zero matches we apply +1 smoothing on numerator and denominator
// (NIST "method 1") so short candidates still produce a non-zero score when
// any unigram matches.
func BLEU4(candidate, reference string) float64 {
	c := tokenize(candidate)
	r := tokenize(reference)
	if len(c) == 0 || len(r) == 0 {
		return 0
	}
	const N = 4
	weights := []float64{0.25, 0.25, 0.25, 0.25}
	var logSum float64
	for n := 1; n <= N; n++ {
		match, total := modifiedPrecision(c, r, n)
		if total == 0 {
			// Candidate too short for this n-gram order — treat as smoothed 0.
			match, total = 1, 1
		}
		if match == 0 {
			match = 1 // +1 smoothing
			total++
		}
		p := float64(match) / float64(total)
		logSum += weights[n-1] * math.Log(p)
	}
	bp := brevityPenalty(len(c), len(r))
	return bp * math.Exp(logSum) * 100
}

func modifiedPrecision(c, r []string, n int) (int, int) {
	cCounts := ngramCounts(c, n)
	rCounts := ngramCounts(r, n)
	var match, total int
	for ng, count := range cCounts {
		total += count
		if rc, ok := rCounts[ng]; ok {
			if count < rc {
				match += count
			} else {
				match += rc
			}
		}
	}
	return match, total
}

func ngramCounts(tokens []string, n int) map[string]int {
	counts := map[string]int{}
	if len(tokens) < n {
		return counts
	}
	for i := 0; i+n <= len(tokens); i++ {
		key := joinNgram(tokens[i : i+n])
		counts[key]++
	}
	return counts
}

func joinNgram(words []string) string {
	out := ""
	for i, w := range words {
		if i > 0 {
			out += "\x00"
		}
		out += w
	}
	return out
}

func brevityPenalty(c, r int) float64 {
	if c >= r {
		return 1
	}
	return math.Exp(1 - float64(r)/float64(c))
}
