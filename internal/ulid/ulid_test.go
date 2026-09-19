package ulid

import "testing"

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	var prev string
	for i := 0; i < 10000; i++ {
		id := New()
		if len(id) != 26 {
			t.Fatalf("len=%d id=%q", len(id), id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
		if prev != "" && id <= prev {
			t.Fatalf("not monotonic: prev=%q id=%q", prev, id)
		}
		prev = id
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
				t.Fatalf("bad char %q in %q", c, id)
			}
		}
	}
}
