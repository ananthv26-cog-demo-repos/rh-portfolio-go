package money

import (
	"errors"
	"fmt"
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

func Parse(text string) (Decimal, error) {
	if text == "" {
		return Decimal{}, ErrInvalidDecimal
	}
	negative := false
	if text[0] == '-' || text[0] == '+' {
		negative = text[0] == '-'
		text = text[1:]
	}
	if text == "" || strings.Count(text, ".") > 1 {
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
	if !allDigits(whole) || !allDigits(fraction) {
		return Decimal{}, ErrInvalidDecimal
	}
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
	return Decimal{coefficient: coefficient, scale: len(fraction), negativeZero: negative && coefficient.Sign() == 0}, nil
}

func (d Decimal) Add(other Decimal) Decimal {
	scale := d.scale
	if other.scale > scale {
		scale = other.scale
	}
	left := scaledCoefficient(d, scale)
	right := scaledCoefficient(other, scale)
	left.Add(left, right)
	return Decimal{coefficient: left, scale: scale, negativeZero: left.Sign() == 0 && (d.negativeZero || other.negativeZero)}
}

func (d Decimal) Sub(other Decimal) Decimal {
	negated := other
	negated.coefficient = new(big.Int).Neg(other.coefficient)
	negated.negativeZero = !other.negativeZero
	return d.Add(negated)
}

func (d Decimal) MulInt64(value int64) Decimal {
	coefficient := new(big.Int).Mul(d.coefficient, big.NewInt(value))
	return Decimal{coefficient: coefficient, scale: d.scale, negativeZero: coefficient.Sign() == 0 && (d.negativeZero != (value < 0))}
}

func (d Decimal) Quantize(scale int) Decimal {
	if scale >= d.scale {
		coefficient := new(big.Int).Mul(d.coefficient, tenTo(scale-d.scale))
		return Decimal{coefficient: coefficient, scale: scale, negativeZero: coefficient.Sign() == 0 && d.negativeZero}
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

func allDigits(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
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

func (d Decimal) GoString() string {
	return fmt.Sprintf("Decimal{%s}", d.String())
}
