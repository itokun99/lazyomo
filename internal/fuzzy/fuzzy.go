package fuzzy

import (
	"sort"
	"unicode/utf8"
)

const (
	ScoreMatch       = 16
	BonusStart       = 16
	BonusAfterSlash  = 12
	BonusAfterSep    = 8
	BonusCamel       = 8
	BonusAlphaDigit  = 4
	BonusConsecutive = 4
	PenaltyGapStart  = -3
	PenaltyGapExtend = -1
	BonusExact       = 1000

	MaxCandidateBytes = 128
	MaxQueryRunes     = 64

	GroupAlias = 0
	GroupRef   = 1
)

const negInf = -1 << 30

type Candidate struct {
	Value string
	Fold  string
	Bonus []int
	Group int
}

type Result struct {
	Candidate Candidate
	Score     int
	Index     int
}

func Fold(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func BonusFor(value string) []int {
	rs := []rune(value)
	out := make([]int, len(rs))
	for i, r := range rs {
		if i == 0 {
			out[i] = BonusStart
			continue
		}
		p := rs[i-1]
		switch {
		case p == '/':
			out[i] = BonusAfterSlash
		case p == ':' || p == '.' || p == '-' || p == '_':
			out[i] = BonusAfterSep
		case isLower(p) && isUpper(r):
			out[i] = BonusCamel
		case isAlphaNum(p) != 0 && isAlphaNum(r) != 0 && isAlphaNum(p) != isAlphaNum(r):
			out[i] = BonusAlphaDigit
		default:
			out[i] = 0
		}
	}
	return out
}

func isLower(r rune) bool { return 'a' <= r && r <= 'z' }
func isUpper(r rune) bool { return 'A' <= r && r <= 'Z' }

func isAlphaNum(r rune) int {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z':
		return 1
	case '0' <= r && r <= '9':
		return 2
	default:
		return 0
	}
}

func NewCandidate(value string, group int) Candidate {
	return Candidate{
		Value: value,
		Fold:  Fold(value),
		Bonus: BonusFor(value),
		Group: group,
	}
}

func truncateCandidate(c Candidate, maxBytes int) ([]rune, []int) {
	if len(c.Fold) <= maxBytes {
		return []rune(c.Fold), normBonus(c.Bonus, utf8.RuneCountInString(c.Fold))
	}
	fr := []rune(c.Fold)
	bytes := 0
	n := 0
	for _, r := range fr {
		sz := utf8.RuneLen(r)
		if bytes+sz > maxBytes {
			break
		}
		bytes += sz
		n++
	}
	return fr[:n], normBonus(c.Bonus, n)
}

func normBonus(b []int, n int) []int {
	if len(b) == n {
		return b
	}
	out := make([]int, n)
	copy(out, b)
	return out
}

func maskOf(rs []rune) uint64 {
	var m uint64
	for _, r := range rs {
		m |= 1 << (uint64(uint32(r)) % 64)
	}
	return m
}

func isSubsequence(c, q []rune) bool {
	j := 0
	for i := 0; i < len(c) && j < len(q); i++ {
		if c[i] == q[j] {
			j++
		}
	}
	return j == len(q)
}

func Score(c Candidate, query string) (int, bool) {
	qf := Fold(query)
	if qf == "" {
		return 0, true
	}
	qr := []rune(qf)
	if len(qr) > MaxQueryRunes {
		return 0, false
	}
	cr, bonus := truncateCandidate(c, MaxCandidateBytes)
	if len(qr) > len(cr) {
		return 0, false
	}
	if maskOf(cr)&maskOf(qr) != maskOf(qr) {
		return 0, false
	}
	if !isSubsequence(cr, qr) {
		return 0, false
	}
	n, m := len(cr), len(qr)
	prev := make([]int, n)
	for j := 0; j < n; j++ {
		if cr[j] == qr[0] {
			prev[j] = ScoreMatch + bonus[j]
		} else {
			prev[j] = negInf
		}
	}
	if m == 1 {
		best := negInf
		for _, v := range prev {
			if v > best {
				best = v
			}
		}
		if best == negInf {
			return 0, false
		}
		if c.Fold == qf {
			best += BonusExact
		}
		return best, true
	}
	curr := make([]int, n)
	pref := make([]int, n)
	for i := 1; i < m; i++ {
		pref[0] = negAdd(prev[0], 0)
		for j := 1; j < n; j++ {
			v := negAdd(prev[j], j)
			if pref[j-1] > v {
				pref[j] = pref[j-1]
			} else {
				pref[j] = v
			}
		}
		for j := 0; j < n; j++ {
			if cr[j] != qr[i] {
				curr[j] = negInf
				continue
			}
			best := negInf
			if j > 0 && prev[j-1] != negInf {
				best = prev[j-1] + BonusConsecutive
			}
			if j >= 2 && pref[j-2] != negInf {
				if g := pref[j-2] - 1 - j; g > best {
					best = g
				}
			}
			if best == negInf {
				curr[j] = negInf
			} else {
				curr[j] = ScoreMatch + bonus[j] + best
			}
		}
		prev, curr = curr, prev
	}
	best := negInf
	for _, v := range prev {
		if v > best {
			best = v
		}
	}
	if best == negInf {
		return 0, false
	}
	if c.Fold == qf {
		best += BonusExact
	}
	return best, true
}

func negAdd(v, k int) int {
	if v == negInf {
		return negInf
	}
	return v + k
}

func Filter(cands []Candidate, query string) []Result {
	if query == "" {
		out := make([]Result, 0, len(cands))
		for i, c := range cands {
			out = append(out, Result{Candidate: c, Score: 0, Index: i})
		}
		sort.SliceStable(out, func(a, b int) bool {
			la, lb := utf8.RuneCountInString(out[a].Candidate.Value), utf8.RuneCountInString(out[b].Candidate.Value)
			if la != lb {
				return la < lb
			}
			return out[a].Index < out[b].Index
		})
		return out
	}
	out := make([]Result, 0, len(cands))
	for i, c := range cands {
		s, ok := Score(c, query)
		if !ok {
			continue
		}
		out = append(out, Result{Candidate: c, Score: s, Index: i})
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		la, lb := utf8.RuneCountInString(out[a].Candidate.Value), utf8.RuneCountInString(out[b].Candidate.Value)
		if la != lb {
			return la < lb
		}
		return out[a].Index < out[b].Index
	})
	return out
}
