package money

import (
	"bufio"
	"os"
	"strconv"
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
		{"parsed negative zero", "-0", "-0.00", 2},
		{"parsed positive zero", "0", "0.00", 2},
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

func TestDecimalDigitMatchesCPythonUnicodeDecimalTable(t *testing.T) {
	file, err := os.Open("testdata/unicode_decimal_digits.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "#") {
			continue
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 2 {
			t.Fatalf("malformed golden row %q", scanner.Text())
		}
		codePoint, err := strconv.ParseInt(strings.TrimPrefix(fields[0], "U+"), 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		want, err := strconv.Atoi(fields[1])
		if err != nil {
			t.Fatal(err)
		}
		got, ok := decimalDigit(rune(codePoint))
		if !ok || got != want {
			t.Fatalf("decimalDigit(%s) = (%d, %t), want (%d, true)",
				fields[0], got, ok, want)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestParseMarkUnicodeDecimalDigits(t *testing.T) {
	price, _ := Parse("19.990000")
	tests := []struct {
		name string
		mark string
		want string
	}{
		{"fullwidth", "２０", "0.10"},
		{"arabic indic", "٢٠", "0.10"},
		{"mathematical sans", "𝟚𝟘", "0.10"},
		{"mixed script", "２0.5", "5.10"},
		{"unicode exponent", "１e２", "800.10"},
		{"unicode underscore neighbour", "１_０", "-99.90"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mark, err := ParseMark(test.mark)
			if err != nil {
				t.Fatal(err)
			}
			got, err := mark.Value.Sub(price).MulInt64(10).StringFixed(2)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("unrealized P&L = %q, want %q", got, test.want)
			}
		})
	}
	t.Run("NaN payload", func(t *testing.T) {
		mark, err := ParseMark("NaN２")
		if err != nil {
			t.Fatal(err)
		}
		if got := mark.NaNString(); got != "NaN2" {
			t.Fatalf("NaNString() = %q, want %q", got, "NaN2")
		}
	})
	for _, input := range []string{"①", "2．5", "＋２０"} {
		t.Run("reject "+input, func(t *testing.T) {
			if _, err := ParseMark(input); err == nil {
				t.Fatalf("ParseMark(%q) unexpectedly succeeded", input)
			}
		})
	}
}

func TestParseMarkPayloadNaNs(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"NaN", "NaN"},
		{"NaN123", "NaN123"},
		{"nan123", "NaN123"},
		{"-NaN007", "-NaN7"},
		{"NaN0", "NaN"},
		{"+NaN12", "NaN12"},
		{"nan1_2", "NaN12"},
		{"NaN123456789012345678901234567890", "NaN3456789012345678901234567890"},
		{"NaN10000000000000000000000000000", "NaN"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			mark, err := ParseMark(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := mark.NaNString(); got != test.want {
				t.Fatalf("NaNString() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUnrealizedPnlPrecisionWindow(t *testing.T) {
	price, _ := Parse("2.675")
	tests := []struct {
		mark string
		want string
	}{
		{"1e-400", "-2.68"},
		{"0", "-2.68"},
		{"-1e-400", "-2.68"},
		{"1e-90", "-2.68"},
		{"1e-27", "-2.67"},
		{"1e-26", "-2.67"},
		{"1e-29", "-2.68"},
		{"1e-85", "-2.68"},
	}
	for _, test := range tests {
		t.Run(test.mark, func(t *testing.T) {
			mark, err := ParseMark(test.mark)
			if err != nil {
				t.Fatal(err)
			}
			got, err := mark.Value.Sub(price).StringFixed(2)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("unrealized P&L = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSignedZeroArithmetic(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"negative zero plus zero", mustDecimalString(t, "-0", 0, false), "0"},
		{"zero plus negative zero", mustDecimalString(t, "0", 0, true), "0"},
		{"negative zero plus negative zero", mustDecimalString(t, "-0", 0, true), "-0"},
		{"negative zero minus zero", mustDecimalString(t, "-0", 1, false), "-0"},
		{"negative five times zero", mustDecimalString(t, "-5", 2, false), "-0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("result = %q, want %q", test.got, test.want)
			}
		})
	}
}

func mustDecimalString(t *testing.T, input string, operation int, otherNegativeZero bool) string {
	t.Helper()
	value, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	switch operation {
	case 0:
		other := Decimal{}
		if otherNegativeZero {
			other, err = Parse("-0")
		} else {
			other, err = Parse("0")
		}
		if err != nil {
			t.Fatal(err)
		}
		return value.Add(other).String()
	case 1:
		other, err := Parse("0")
		if err != nil {
			t.Fatal(err)
		}
		return value.Sub(other).String()
	default:
		return value.MulInt64(0).String()
	}
}

func TestParseMarkRejectsInvalidNaNPayloads(t *testing.T) {
	for _, input := range []string{"sNaN12", "-sNaN", "NaN12x", "NaN_12", "NaN12_"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseMark(input); err == nil {
				t.Fatal("expected an error")
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

func TestParseRejectsExponentWithoutMantissa(t *testing.T) {
	for _, input := range []string{"e5", "-e5", "+e-3"} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseMark(input); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
	if _, err := ParseMark(".e3"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParseExponentMatchesCPythonLimits(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"1e-1999999999999999997", true},
		{"1.0e-1999999999999999996", true},
		{"1.23e-1999999999999999995", true},
		{"0.001e-1999999999999999994", true},
		{"1e-1999999999999999998", false},
		{"1e999999999999999999", true},
		{"12e999999999999999998", true},
		{"123e999999999999999997", true},
		{"1e1000000000000000000", false},
		{"1e-9223372036854775807", false},
		{"1.0e-9223372036854775807", false},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			_, err := ParseMark(test.input)
			if (err == nil) != test.ok {
				t.Fatalf("error = %v, want accepted=%v", err, test.ok)
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

func TestNarrowingQuantizePreservesSignedZero(t *testing.T) {
	value, err := Parse("-0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := value.Quantize(0)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := got.StringFixed(2)
	if err != nil {
		t.Fatal(err)
	}
	if formatted != "-0.00" {
		t.Fatalf("narrowed signed zero = %q", formatted)
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

func TestResidualDirectionOpposingNegativeValueRoundsTowardZero(t *testing.T) {
	mark, err := Parse("0.005" + strings.Repeat("0", 80) + "1")
	if err != nil {
		t.Fatal(err)
	}
	cost, err := Parse("0.01")
	if err != nil {
		t.Fatal(err)
	}
	got, err := mark.Sub(cost).StringFixed(2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "-0.00" {
		t.Fatalf("opposing residual direction = %q", got)
	}
}

func TestResidualDirectionOpposingOddQuotientMatchesContextRounding(t *testing.T) {
	mark, err := Parse("0.005" + strings.Repeat("0", 80) + "1")
	if err != nil {
		t.Fatal(err)
	}
	cost, err := Parse("0.02")
	if err != nil {
		t.Fatal(err)
	}
	got, err := mark.Sub(cost).StringFixed(2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "-0.02" {
		t.Fatalf("context-rounded odd opposing residual = %q, want -0.02", got)
	}
}

func TestArithmeticRoundsResidualBeforeQuantize(t *testing.T) {
	mark, err := Parse("2.680" + strings.Repeat("0", 80) + "1")
	if err != nil {
		t.Fatal(err)
	}
	cost, err := Parse("2.675000")
	if err != nil {
		t.Fatal(err)
	}
	got, err := mark.Sub(cost).StringFixed(2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.00" {
		t.Fatalf("context-rounded tie = %q, want 0.00", got)
	}
}

func TestCombineResidualOpposingDirectionsCancel(t *testing.T) {
	// Arithmetic clears residuals after rounding, so two inexact operands
	// are currently unreachable through the public money operations.
	if got := combineResidual(1, -1); got != 0 {
		t.Fatalf("opposing residuals = %d, want 0", got)
	}
}
