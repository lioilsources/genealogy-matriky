package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDumpPairs: dump obsahuje všechny páry z blockingu a rule_score se
// shoduje se scorePair — ML baseline tak reprodukuje přesně matcher.
func TestDumpPairs(t *testing.T) {
	db := setupPipeline(t)
	out := filepath.Join(t.TempDir(), "pairs.jsonl")
	if err := runDumpPairs(db, out); err != nil {
		t.Fatal(err)
	}

	ms, err := loadMentions(db)
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]int64]float64{}
	for _, pr := range candidatePairs(ms) {
		s, _ := scorePair(pr[0], pr[1])
		want[[2]int64{pr[0].ID, pr[1].ID}] = s
	}

	fh, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	n, positive := 0, 0
	for sc.Scan() {
		var r dumpRow
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		k := [2]int64{r.A.Mention, r.B.Mention}
		s, ok := want[k]
		if !ok {
			t.Fatalf("pár %v není v candidatePairs", k)
		}
		if s != r.RuleScore {
			t.Errorf("pár %v: rule_score %v, scorePair %v", k, r.RuleScore, s)
		}
		if r.A.Book == "" || r.A.Scan == "" {
			t.Errorf("pár %v: chybí book/scan", k)
		}
		if r.RuleScore == 0 && r.RuleVeto == "" {
			t.Errorf("pár %v: skóre 0 bez důvodu (veto)", k)
		}
		if r.RuleScore > 0 {
			positive++
		}
		n++
	}
	if n != len(want) {
		t.Errorf("dump má %d párů, blocking %d", n, len(want))
	}
	if positive == 0 {
		t.Error("žádný pár s kladným skóre — fixture by měla mít shody")
	}
}
