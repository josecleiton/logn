package legal

import "testing"

func TestNormalizeCountry(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{
		"br":    {"BR", true},
		" US ":  {"US", true},
		"":      {"", true},
		"BRA":   {"BR", true},
		"usa":   {"US", true},
		"MEX":   {"MX", true},
		"ZZ":    {"", false},
		"XYZ":   {"", false},
		"001":   {"", false},
		"B1":    {"", false},
		"'; --": {"", false},
	}
	for in, c := range cases {
		got, ok := NormalizeCountry(in)
		if got != c.want || ok != c.ok {
			t.Errorf("%q: got (%q, %v), want (%q, %v)", in, got, ok, c.want, c.ok)
		}
	}
}

func TestMinimumAge(t *testing.T) {
	if MinimumAge("BR") != DefaultMinimumAge || MinimumAge("") != DefaultMinimumAge {
		t.Fatal("sem entrada na tabela, vale a idade padrão")
	}
	minimumAgeByCountry["XX"] = 16
	defer delete(minimumAgeByCountry, "XX")
	if MinimumAge("XX") != 16 {
		t.Fatal("a tabela manda")
	}
}
