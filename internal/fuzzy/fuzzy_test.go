package fuzzy

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// bruteScore enumerates every increasing alignment of query runes inside the
// candidate and returns the best score using the SAME constants as the
// production scorer. It is intentionally exponential and independent of the
// DP (plain recursion, no prefix optimisation) so agreement proves the DP.
func bruteScore(c Candidate, query string) (int, bool) {
	qf := Fold(query)
	if qf == "" {
		return 0, true
	}
	qr := []rune(qf)
	if len(qr) > MaxQueryRunes {
		return 0, false
	}
	cRunes, bonus := truncateCandidate(c, MaxCandidateBytes)
	if len(qr) > len(cRunes) {
		return 0, false
	}
	if !isSubsequence(cRunes, qr) {
		return 0, false
	}
	best := negInf
	var rec func(qi, start int, prev int, acc int)
	rec = func(qi, start int, prev int, acc int) {
		if qi == len(qr) {
			if acc > best {
				best = acc
			}
			return
		}
		for j := start; j < len(cRunes); j++ {
			if cRunes[j] != qr[qi] {
				continue
			}
			add := ScoreMatch + bonus[j]
			if qi > 0 {
				gap := j - prev - 1
				if gap == 0 {
					add += BonusConsecutive
				} else {
					add += PenaltyGapStart + (gap-1)*PenaltyGapExtend
				}
			}
			rec(qi+1, j+1, j, acc+add)
		}
	}
	rec(0, 0, -1, 0)
	if best == negInf {
		return 0, false
	}
	if c.Fold == qf && len(qr) > 0 {
		best += BonusExact
	}
	return best, true
}

func TestScoreOracle500(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	alpha := []rune("absonABSON12/-_:.mMxXy")
	for i := 0; i < 500; i++ {
		cn := 1 + rng.Intn(12)
		var sb strings.Builder
		for j := 0; j < cn; j++ {
			sb.WriteRune(alpha[rng.Intn(len(alpha))])
		}
		cValue := sb.String()
		group := GroupAlias
		if rng.Intn(2) == 1 {
			group = GroupRef
		}
		c := NewCandidate(cValue, group)
		var q string
		switch rng.Intn(4) {
		case 0:
			// random subsequence of the candidate (likely match)
			cr := []rune(c.Fold)
			var qb strings.Builder
			for _, r := range cr {
				if rng.Intn(2) == 0 {
					qb.WriteRune(r)
				}
			}
			q = qb.String()
		case 1:
			// short random string (may or may not match)
			qn := rng.Intn(5)
			var qb strings.Builder
			for j := 0; j < qn; j++ {
				qb.WriteRune(alpha[rng.Intn(len(alpha))])
			}
			q = qb.String()
		case 2:
			// uppercase variant to exercise ASCII fold
			cr := []rune(cValue)
			var qb strings.Builder
			for _, r := range cr {
				if rng.Intn(2) == 0 {
					if 'a' <= r && r <= 'z' {
						r = r - 'a' + 'A'
					}
					qb.WriteRune(r)
				}
			}
			q = qb.String()
		default:
			// query longer than candidate (must not match)
			q = cValue + "zzzqqq"
		}
		gotScore, gotOK := Score(c, q)
		wantScore, wantOK := bruteScore(c, q)
		if gotOK != wantOK || gotScore != wantScore {
			t.Fatalf("pair %d: candidate %q query %q: Score=%d,%v want brute=%d,%v", i, cValue, q, gotScore, gotOK, wantScore, wantOK)
		}
	}
}

func TestFilterRankingSon(t *testing.T) {
	cands := []Candidate{
		NewCandidate("sonoma", GroupAlias),
		NewCandidate("sonnet", GroupAlias),
		NewCandidate("son", GroupAlias),
		NewCandidate("sxxon", GroupAlias),
		NewCandidate("a/son", GroupRef),
		NewCandidate("ason", GroupRef),
	}
	got := Filter(cands, "son")
	if len(got) == 0 {
		t.Fatalf("Filter returned no results for query %q", "son")
	}
	pos := map[string]int{}
	for i, r := range got {
		pos[r.Candidate.Value] = i
	}
	// exact beats everything via +1000
	if pos["son"] != 0 {
		t.Fatalf("exact %q should rank first, got order %v", "son", orderOf(got))
	}
	// sonoma precedes sonnet on identical score+length via index tie-break
	if pos["sonoma"] >= pos["sonnet"] {
		t.Fatalf("sonoma should rank before sonnet (tie-break by index), got %v", orderOf(got))
	}
	// consecutive start beats gapped middle
	if pos["sonoma"] >= pos["sxxon"] {
		t.Fatalf("consecutive sonoma should beat gapped sxxon, got %v", orderOf(got))
	}
	// afterSlash bonus beats plain
	if pos["a/son"] >= pos["ason"] {
		t.Fatalf("afterSlash a/son should beat ason, got %v", orderOf(got))
	}
	// determinism: repeat yields identical order
	got2 := Filter(cands, "son")
	for i := range got {
		if got[i].Candidate.Value != got2[i].Candidate.Value || got[i].Score != got2[i].Score {
			t.Fatalf("non-deterministic order: %v vs %v", orderOf(got), orderOf(got2))
		}
	}
}

func orderOf(rs []Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = fmt.Sprintf("%s:%d", r.Candidate.Value, r.Score)
	}
	return out
}

func TestFilterEmptyQuery(t *testing.T) {
	cands := []Candidate{
		NewCandidate("longer-name-here", GroupAlias),
		NewCandidate("son", GroupAlias),
		NewCandidate("ab", GroupRef),
	}
	got := Filter(cands, "son"[:0])
	if len(got) != len(cands) {
		t.Fatalf("empty query should return full catalog (%d), got %d", len(cands), len(got))
	}
	for _, r := range got {
		if r.Score != 0 {
			t.Fatalf("empty query score should be 0, got %d for %q", r.Score, r.Candidate.Value)
		}
	}
	// tie-break: rune length asc then index
	if got[0].Candidate.Value != "ab" || got[1].Candidate.Value != "son" {
		t.Fatalf("empty query tie-break should be length asc, got %v", orderOf(got))
	}
}

func TestFilterEdgeCases(t *testing.T) {
	// non-ASCII query matches literally (ASCII fold leaves it intact)
	c := NewCandidate("caf\u00e9-son", GroupAlias)
	if _, ok := Score(c, "\u00e9-s"); !ok {
		t.Fatalf("non-ASCII query should match literally")
	}
	if _, ok := Score(c, "\u00c9-S"); ok {
		t.Fatalf("non-ASCII case must NOT fold (no smart case): upper \u00c9 should not match lower \u00e9")
	}
	// ASCII fold is case-insensitive
	c2 := NewCandidate("SonOMA", GroupAlias)
	s1, ok1 := Score(c2, "son")
	s2, ok2 := Score(c2, "SON")
	if !ok1 || !ok2 || s1 != s2 {
		t.Fatalf("ASCII fold must be case-insensitive: %d,%v vs %d,%v", s1, ok1, s2, ok2)
	}
	// query longer than candidate cannot match
	c3 := NewCandidate("ab", GroupAlias)
	if _, ok := Score(c3, "abcdef"); ok {
		t.Fatalf("query longer than candidate must not match")
	}
	// empty candidate never matches a non-empty query
	c4 := NewCandidate("", GroupAlias)
	if _, ok := Score(c4, "a"); ok {
		t.Fatalf("empty candidate must not match non-empty query")
	}
	// camel / digit / separator bonuses are observable
	camel := NewCandidate("fooBar", GroupAlias)
	plain := NewCandidate("foobar", GroupAlias)
	sc, _ := Score(camel, "b")
	sp, _ := Score(plain, "b")
	if sc <= sp {
		t.Fatalf("camel bonus should make fooBar(%d) beat foobar(%d) for query b", sc, sp)
	}
	slash := NewCandidate("a/b", GroupRef)
	noslash := NewCandidate("ab", GroupRef)
	ss, _ := Score(slash, "b")
	sn, _ := Score(noslash, "b")
	if ss <= sn {
		t.Fatalf("afterSlash bonus should make a/b(%d) beat ab(%d)", ss, sn)
	}
	digit := NewCandidate("a1b", GroupAlias)
	nodigit := NewCandidate("axb", GroupAlias)
	sd, _ := Score(digit, "b")
	sx, _ := Score(nodigit, "b")
	if sd <= sx {
		t.Fatalf("alphaDigit bonus should make a1b(%d) beat axb(%d)", sd, sx)
	}
	// caps: query beyond 64 runes never matches
	longQ := strings.Repeat("a", MaxQueryRunes+1)
	if _, ok := Score(NewCandidate("aaa", GroupAlias), longQ); ok {
		t.Fatalf("query longer than %d runes must not match", MaxQueryRunes)
	}
	// candidate beyond 128 bytes is truncated at a rune boundary
	big := strings.Repeat("a", MaxCandidateBytes+10)
	cb := NewCandidate(big, GroupAlias)
	if got := utf8.RuneCountInString(big); got == 0 {
		t.Fatalf("test setup broken")
	}
	if _, ok := Score(cb, "a"); !ok {
		t.Fatalf("truncated candidate should still match single-char query")
	}
}

func TestFilter20kBudget(t *testing.T) {
	cands := buildBenchCandidates(20000)
	start := time.Now()
	got := Filter(cands, "son")
	el := time.Since(start)
	t.Logf("Filter20k: %d results in %v", len(got), el)
	if el > 100*time.Millisecond {
		t.Fatalf("budget exceeded: Filter over 20k took %v (>100ms)", el)
	}
	if len(got) == 0 {
		t.Fatalf("expected some matches for query son over 20k candidates")
	}
}

func BenchmarkFilter20k(b *testing.B) {
	cands := buildBenchCandidates(20000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Filter(cands, "son")
	}
}

func buildBenchCandidates(n int) []Candidate {
	providers := []string{"adacode", "github-copilot", "opencode", "sonoma-labs"}
	models := []string{"sonoma-1", "sonnet-5", "k3-max", "space-bunny", "FooBar-2", "a/b-test", "model-1x"}
	out := make([]Candidate, 0, n)
	for i := 0; i < n; i++ {
		v := fmt.Sprintf("%s/%s-%d", providers[i%len(providers)], models[i%len(models)], i)
		g := GroupRef
		if i%3 == 0 {
			g = GroupAlias
		}
		out = append(out, NewCandidate(v, g))
	}
	return out
}
