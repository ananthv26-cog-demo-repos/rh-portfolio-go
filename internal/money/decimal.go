package money

import (
	"errors"
	"math/big"
	"strings"
)

var ErrInvalidDecimal = errors.New("invalid decimal")

// Decimal stores an exact decimal as an integer coefficient and a decimal scale.
type Decimal struct {
	coefficient  *big.Int
	scale        int
	negativeZero bool
}

type Mark struct {
	Value       Decimal
	NaN         bool
	NegativeNaN bool
}

func ParseMark(text string) (Mark, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Mark{}, ErrInvalidDecimal
	}
	sign := byte(0)
	if text[0] == '-' || text[0] == '+' {
		sign, text = text[0], text[1:]
	}
	switch strings.ToLower(text) {
	case "nan":
		return Mark{NaN: true, NegativeNaN: sign == '-'}, nil
	case "snan", "inf", "infinity":
		return Mark{}, ErrInvalidDecimal
	default:
		value, err := Parse(stringWithSign(sign, text))
		return Mark{Value: value}, err
	}
}

func Parse(text string) (Decimal, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Decimal{}, ErrInvalidDecimal
	}
	negative := false
	if text[0] == '-' || text[0] == '+' {
		negative = text[0] == '-'
		text = text[1:]
	}
	if text == "" {
		return Decimal{}, ErrInvalidDecimal
	}
	exponent := 0
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		if strings.IndexAny(text[index+1:], "eE") >= 0 {
			return Decimal{}, ErrInvalidDecimal
		}
		var ok bool
		exponent, ok = parseExponent(text[index+1:])
		if !ok {
			return Decimal{}, ErrInvalidDecimal
		}
		text = text[:index]
	}
	if strings.Count(text, ".") > 1 {
		return Decimal{}, ErrInvalidDecimal
	}
	parts := strings.SplitN(text, ".", 2)
	whole, fraction := parts[0], ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if whole == "" {
		whole = "0"
	}
	if len(parts) == 2 && parts[0] == "" && parts[1] == "" {
		return Decimal{}, ErrInvalidDecimal
	}
	if !validDigitSeparators(whole) || !validDigitSeparators(fraction) {
		return Decimal{}, ErrInvalidDecimal
	}
	whole = strings.ReplaceAll(whole, "_", "")
	fraction = strings.ReplaceAll(fraction, "_", "")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		digits = "0"
	}
	coefficient, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return Decimal{}, ErrInvalidDecimal
	}
	if negative {
		coefficient.Neg(coefficient)
	}
	scale := len(fraction) - exponent
	if scale < 0 {
		coefficient.Mul(coefficient, tenTo(-scale))
		scale = 0
	}
	return Decimal{coefficient: coefficient, scale: scale}, nil
}

func stringWithSign(sign byte, value string) string {
	if sign == 0 {
		return value
	}
	return string(sign) + value
}

func (d Decimal) Add(other Decimal) Decimal {
	scale := d.scale
	if other.scale > scale {
		scale = other.scale
	}
	left := scaledCoefficient(d, scale)
	right := scaledCoefficient(other, scale)
	left.Add(left, right)
	return Decimal{coefficient: left, scale: scale}
}

func (d Decimal) Sub(other Decimal) Decimal {
	scale := d.scale
	if other.scale > scale {
		scale = other.scale
	}
	left := scaledCoefficient(d, scale)
	right := scaledCoefficient(other, scale)
	left.Sub(left, right)
	return Decimal{coefficient: left, scale: scale}
}

func (d Decimal) MulInt64(value int64) Decimal {
	coefficient := new(big.Int).Mul(d.coefficient, big.NewInt(value))
	return Decimal{coefficient: coefficient, scale: d.scale}
}

func (d Decimal) Quantize(scale int) Decimal {
	if scale >= d.scale {
		coefficient := new(big.Int).Mul(d.coefficient, tenTo(scale-d.scale))
		return Decimal{coefficient: coefficient, scale: scale, negativeZero: d.negativeZero}
	}
	return roundRatio(d.coefficient, tenTo(d.scale-scale), scale)
}

// DivideQuantized divides d by divisor and rounds the result to scale places.
func (d Decimal) DivideQuantized(divisor int64, scale int) Decimal {
	if divisor == 0 {
		panic("division by zero")
	}
	numerator := new(big.Int).Mul(d.coefficient, tenTo(scale))
	denominator := new(big.Int).Mul(big.NewInt(divisor), tenTo(d.scale))
	if denominator.Sign() < 0 {
		numerator.Neg(numerator)
		denominator.Neg(denominator)
	}
	return roundRatio(numerator, denominator, scale)
}

func (d Decimal) StringFixed(scale int) string {
	quantized := d.Quantize(scale)
	negative := quantized.coefficient.Sign() < 0 || (quantized.coefficient.Sign() == 0 && quantized.negativeZero)
	absolute := new(big.Int).Abs(quantized.coefficient)
	digits := absolute.String()
	if scale == 0 {
		if negative {
			return "-" + digits
		}
		return digits
	}
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale+1-len(digits)) + digits
	}
	result := digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
	if negative {
		return "-" + result
	}
	return result
}

func (d Decimal) String() string {
	return d.StringFixed(d.scale)
}

func validDigitSeparators(value string) bool {
	for index, char := range value {
		if char == '_' {
			if index == 0 || index == len(value)-1 || value[index-1] < '0' || value[index-1] > '9' ||
				value[index+1] < '0' || value[index+1] > '9' {
				return false
			}
			continue
		}
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func parseExponent(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	sign := 1
	if value[0] == '+' || value[0] == '-' {
		if value[0] == '-' {
			sign = -1
		}
		value = value[1:]
	}
	if !validDigitSeparators(value) {
		return 0, false
	}
	value = strings.ReplaceAll(value, "_", "")
	exponent := new(big.Int)
	if _, ok := exponent.SetString(value, 10); !ok || !exponent.IsInt64() {
		return 0, false
	}
	return sign * int(exponent.Int64()), true
}

func scaledCoefficient(value Decimal, scale int) *big.Int {
	return new(big.Int).Mul(value.coefficient, tenTo(scale-value.scale))
}

func tenTo(scale int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
}

func roundRatio(numerator, denominator *big.Int, scale int) Decimal {
	if denominator.Sign() == 0 {
		panic("division by zero")
	}
	negative := numerator.Sign() < 0
	absoluteNumerator := new(big.Int).Abs(numerator)
	absoluteDenominator := new(big.Int).Abs(denominator)
	quotient, remainder := new(big.Int).QuoRem(absoluteNumerator, absoluteDenominator, new(big.Int))
	twiceRemainder := new(big.Int).Lsh(remainder, 1)
	if twiceRemainder.Cmp(absoluteDenominator) > 0 ||
		(twiceRemainder.Cmp(absoluteDenominator) == 0 && quotient.Bit(0) == 1) {
		quotient.Add(quotient, big.NewInt(1))
	}
	if negative {
		quotient.Neg(quotient)
	}
	return Decimal{coefficient: quotient, scale: scale, negativeZero: negative && quotient.Sign() == 0}
}
