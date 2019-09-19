package lanhttp

import (
	"math/rand"
	"testing"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	type testcase struct {
		haveA Routes
		haveB Routes
		want  bool
	}
	tcs := map[string]testcase{
		"same empty": testcase{
			haveA: Routes{},
			haveB: Routes{},
			want:  false,
		},
		"same content": testcase{
			haveA: Routes{"a": []string{"a"}},
			haveB: Routes{"a": []string{"a"}},
			want:  false,
		},
		"sort": testcase{
			haveA: Routes{"a": []string{"a", "b"}},
			haveB: Routes{"a": []string{"b", "a"}},
			want:  false,
		},
		"a > b": testcase{
			haveA: Routes{"a": []string{"a"}},
			haveB: Routes{},
			want:  true,
		},
		"b > a": testcase{
			haveA: Routes{},
			haveB: Routes{"a": []string{"a"}},
			want:  true,
		},
		"a > b ips": testcase{
			haveA: Routes{"a": []string{"a", "b", "c"}},
			haveB: Routes{"a": []string{"a", "c"}},
			want:  true,
		},
		"b > a ips": testcase{
			haveA: Routes{"a": []string{"a"}},
			haveB: Routes{"a": []string{"a", "b"}},
			want:  true,
		},
		"a != b": testcase{
			haveA: Routes{"a": []string{"a"}},
			haveB: Routes{"b": []string{"a"}},
			want:  true,
		},
		"a != b ips": testcase{
			haveA: Routes{"a": []string{"a"}},
			haveB: Routes{"a": []string{"b"}},
			want:  true,
		},
	}
	for name, tc := range tcs {
		tc := tc // capture reference
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if diff(tc.haveA, tc.haveB) != tc.want {
				t.Fatal(name, tc.haveA, tc.haveB)
			}
		})
	}
}

func TestGetIP(t *testing.T) {
	// Set a seed to ensure our results below are consistent
	rand.Seed(16)
	c := NewClient(nil).WithRoutes(Routes{
		"a.internal": []string{"1", "2"},
	})
	if got := c.getIP("a.internal"); got != "1" {
		t.Fatal("expected 1 (1st)")
	}
	if got := c.getIP("a.internal"); got != "2" {
		t.Fatal("expected 2 (1st)")
	}
	if got := c.getIP("a.internal"); got != "1" {
		t.Fatal("expected 1 (2nd)")
	}
	if got := c.getIP("a.internal"); got != "2" {
		t.Fatal("expected 2 (2nd)")
	}
}
