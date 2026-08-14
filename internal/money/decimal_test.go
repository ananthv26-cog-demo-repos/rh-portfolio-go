package money

import "testing"

func TestFixtureValues(t *testing.T) {
	tests := []struct {
		name, input, want string
		scale             int
	}{
		{"p1 average", "19.99", "19.9900", 4},
		{"p1 pnl", "0.10", "0.10", 2},
		{"p4 pnl", "-199.90", "-199.90", 2},
		{"p5 tie", "1.234650", "1.2346", 4},
		{"p6 tie", "-2.675", "-2.68", 2},
		{"p7 signed zero", "-0.0002", "-0.00", 2},
		{"p9 pnl", "100000", "100000.00", 2},
		{"p11 negative mark", "-214.90", "-214.90", 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := Parse(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := value.StringFixed(test.scale); got != test.want {
				t.Fatalf("StringFixed(%d) = %q, want %q", test.scale, got, test.want)
			}
		})
	}
}

func TestDivideQuantizedHalfEven(t *testing.T) {
	value, _ := Parse("2.469300")
	if got := value.DivideQuantized(2, 4).StringFixed(4); got != "1.2346" {
		t.Fatalf("got %s", got)
	}
	value, _ = Parse("11")
	if got := value.DivideQuantized(7, 4).StringFixed(4); got != "1.5714" {
		t.Fatalf("got %s", got)
	}
}
