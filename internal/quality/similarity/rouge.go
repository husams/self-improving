package similarity

// ROUGE_L returns the F-measure of the Longest Common Subsequence between
// candidate and reference, scaled to 0-100. Pure-Go, no external deps.
//
// Definitions (Lin, 2004):
//   precision = LCS(c, r) / len(c)
//   recall    = LCS(c, r) / len(r)
//   F-beta    = ((1+beta^2) * P * R) / (R + beta^2 * P)
//
// We use beta=1 (F1) — symmetric precision/recall trade-off, the standard
// choice for ROUGE-L when no length-bias is desired.
func ROUGE_L(candidate, reference string) float64 {
	c := tokenize(candidate)
	r := tokenize(reference)
	if len(c) == 0 || len(r) == 0 {
		return 0
	}
	lcs := lcsLen(c, r)
	if lcs == 0 {
		return 0
	}
	p := float64(lcs) / float64(len(c))
	rec := float64(lcs) / float64(len(r))
	if p+rec == 0 {
		return 0
	}
	f := (2 * p * rec) / (p + rec)
	return f * 100
}

func lcsLen(a, b []string) int {
	n, m := len(a), len(b)
	prev := make([]int, m+1)
	curr := make([]int, m+1)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] >= curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
		for j := range curr {
			curr[j] = 0
		}
	}
	return prev[m]
}
