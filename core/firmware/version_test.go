package firmware

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2601.1", "2512", 1}, {"v6.4.1", "6.4.1", 0}, {"1.10", "1.9", 1}, {"1.2", "1.2.0", 0}, {"4.0", "4.0.1", -1},
		{"1.02", "1.2", 0}, {"99999999999999999999.1", "99999999999999999998.9", 1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("%s vs %s = %d", c.a, c.b, got)
		}
	}
}

func TestInRange(t *testing.T) {
	if !InRange("2.0", "2.0", "") || InRange("1.9", "2.0", "") || InRange("3.0", "", "3.0") || !InRange("", "", "") || InRange("", "1.0", "") {
		t.Fatal("range")
	}
	if InRange("", "", "1.0") || !InRange("2.5", "2.0", "3.0") {
		t.Fatal("range")
	}
}

func TestPick(t *testing.T) {
	f := Fact{Alts: []Fact{
		{Value: "{card}/ROMS", Until: "2601"},
		{Value: "{card}/Roms", Since: "2601"},
	}}
	if got, ok := f.Pick("2512"); !ok || got.String() != "{card}/ROMS" {
		t.Fatalf("%+v %v", got, ok)
	}
	if got, ok := f.Pick("2601.1"); !ok || got.String() != "{card}/Roms" {
		t.Fatalf("%+v %v", got, ok)
	}
	if _, ok := f.Pick(""); ok {
		t.Fatal("unknown version matched ranged alternatives")
	}
	plain := Fact{Value: "/a"}
	if got, ok := plain.Pick("1"); !ok || got.String() != "/a" {
		t.Fatalf("%+v %v", got, ok)
	}
	if got, ok := plain.Pick(""); !ok || got.String() != "/a" {
		t.Fatalf("%+v %v", got, ok)
	}
	ranged := Fact{Value: "/a", Since: "2"}
	if _, ok := ranged.Pick("1"); ok {
		t.Fatal("out of range")
	}
	gap := Fact{Alts: []Fact{{Value: "/a", Until: "1"}}}
	if _, ok := gap.Pick("5"); ok {
		t.Fatal("gap")
	}
}
