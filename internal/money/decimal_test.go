package money

import (
	"strings"
	"testing"
)

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
			got, err := value.StringFixed(test.scale)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("StringFixed(%d) = %q, want %q", test.scale, got, test.want)
			}
		})
	}
}

func TestDivideQuantizedHalfEven(t *testing.T) {
	value, _ := Parse("2.469300")
	quantized, err := value.DivideQuantized(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	got, err := quantized.StringFixed(4)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2346" {
		t.Fatalf("got %s", got)
	}
	value, _ = Parse("11")
	quantized, err = value.DivideQuantized(7, 4)
	if err != nil {
		t.Fatal(err)
	}
	got, err = quantized.StringFixed(4)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.5714" {
		t.Fatalf("got %s", got)
	}
}

func TestParseMarkPythonDecimalSurface(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		want       string
		wantNaN    bool
		wantNegNaN bool
	}{
		{name: "exponent", input: "2e1", want: "20"},
		{name: "whitespace", input: "  20.00  ", want: "20.00"},
		{name: "plus sign", input: "+20.00", want: "20.00"},
		{name: "underscores", input: "1_0", want: "10"},
		{name: "nan", input: "NaN", want: "NaN", wantNaN: true},
		{name: "negative nan", input: "-nAn", want: "-NaN", wantNaN: true, wantNegNaN: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mark, err := ParseMark(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if mark.NaN != test.wantNaN || mark.NegativeNaN != test.wantNegNaN {
				t.Fatalf("mark = %+v", mark)
			}
			if test.wantNaN {
				return
			}
			if got := mark.Value.String(); got != test.want {
				t.Fatalf("String() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseMarkRejectsNonFiniteValues(t *testing.T) {
	for _, input := range []string{"Infinity", "-Infinity", "sNaN", "+sNaN"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseMark(input); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestNegativeZeroOnlyComesFromRounding(t *testing.T) {
	value, _ := Parse("-0.0002")
	got, err := value.StringFixed(2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "-0.00" {
		t.Fatalf("rounded negative zero = %q", got)
	}
	zero, _ := Parse("0")
	got, err = zero.Sub(zero).StringFixed(2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.00" {
		t.Fatalf("exact zero = %q", got)
	}
}

func TestResidualBeyondRetainedScaleBreaksHalfEvenTieAwayFromZero(t *testing.T) {
	plain, err := Parse("1.23465")
	if err != nil {
		t.Fatal(err)
	}
	got, err := plain.StringFixed(4)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2346" {
		t.Fatalf("plain tie = %q, want 1.2346", got)
	}
	residual, err := Parse("1.23465" + strings.Repeat("0", 80) + "1")
	if err != nil {
		t.Fatal(err)
	}
	got, err = residual.StringFixed(4)
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2347" {
		t.Fatalf("residual tie = %q, want 1.2347", got)
	}
}
