package lanhttp

import (
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
			haveA: Routes{"a": &backend{IPs: []string{"a"}}},
			haveB: Routes{"a": &backend{IPs: []string{"a"}}},
			want:  false,
		},
		"sort": testcase{
			haveA: Routes{"a": &backend{IPs: []string{"a", "b"}}},
			haveB: Routes{"a": &backend{IPs: []string{"b", "a"}}},
			want:  false,
		},
		"index ignored": testcase{
			haveA: Routes{"a": &backend{Index: 0}},
			haveB: Routes{"a": &backend{Index: 1}},
			want:  false,
		},
		"a > b": testcase{
			haveA: Routes{"a": &backend{IPs: []string{"a"}}},
			haveB: Routes{},
			want:  true,
		},
		"b > a": testcase{
			haveA: Routes{},
			haveB: Routes{"a": &backend{IPs: []string{"a"}}},
			want:  true,
		},
		"a > b ips": testcase{
			haveA: Routes{"a": &backend{IPs: []string{"a", "b", "c"}}},
			haveB: Routes{"a": &backend{IPs: []string{"a", "c"}}},
			want:  true,
		},
		"b > a ips": testcase{
			haveA: Routes{"a": &backend{IPs: []string{"a"}}},
			haveB: Routes{"a": &backend{IPs: []string{"a", "b"}}},
			want:  true,
		},
		"a != b": testcase{
			haveA: Routes{"a": &backend{IPs: []string{"a"}}},
			haveB: Routes{"b": &backend{IPs: []string{"a"}}},
			want:  true,
		},
		"a != b ips": testcase{
			haveA: Routes{"a": &backend{IPs: []string{"a"}}},
			haveB: Routes{"a": &backend{IPs: []string{"b"}}},
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
	c := NewClient(nil).WithRoutes(Routes{
		"a.internal": &backend{IPs: []string{"1", "2"}},
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
