package lanhttp

import (
	"testing"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	type testcase struct {
		haveA map[string][]string
		haveB map[string][]string
		want  bool
	}
	tcs := map[string]testcase{
		"same empty": testcase{
			haveA: map[string][]string{},
			haveB: map[string][]string{},
			want:  false,
		},
		"same content": testcase{
			haveA: map[string][]string{"a": []string{"a"}},
			haveB: map[string][]string{"a": []string{"a"}},
			want:  false,
		},
		"sort": testcase{
			haveA: map[string][]string{"a": []string{"a", "b"}},
			haveB: map[string][]string{"a": []string{"b", "a"}},
			want:  false,
		},
		"a > b": testcase{
			haveA: map[string][]string{"a": []string{"a"}},
			haveB: map[string][]string{},
			want:  true,
		},
		"b > a": testcase{
			haveA: map[string][]string{},
			haveB: map[string][]string{"a": []string{"a"}},
			want:  true,
		},
		"a > b ips": testcase{
			haveA: map[string][]string{"a": []string{"a", "b", "c"}},
			haveB: map[string][]string{"a": []string{"a", "c"}},
			want:  true,
		},
		"b > a ips": testcase{
			haveA: map[string][]string{"a": []string{"a"}},
			haveB: map[string][]string{"a": []string{"a", "b"}},
			want:  true,
		},
		"a != b": testcase{
			haveA: map[string][]string{"a": []string{"a"}},
			haveB: map[string][]string{"b": []string{"a"}},
			want:  true,
		},
		"a != b ips": testcase{
			haveA: map[string][]string{"a": []string{"a"}},
			haveB: map[string][]string{"a": []string{"b"}},
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
	c := NewClient(nil)
	c.UpdateRoutes(Routes{
		"a.internal": []string{"1", "2"},
	})
	if got := c.getIP("a.internal"); got != "2" {
		t.Fatal("expected 2 (1st)")
	}
	if got := c.getIP("a.internal"); got != "1" {
		t.Fatal("expected 1 (1st)")
	}
	if got := c.getIP("a.internal"); got != "2" {
		t.Fatal("expected 2 (2nd)")
	}
	if got := c.getIP("a.internal"); got != "1" {
		t.Fatal("expected 1 (2nd)")
	}
}
