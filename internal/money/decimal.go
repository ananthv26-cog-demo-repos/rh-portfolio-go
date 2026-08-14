package money

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
)

var ErrInvalidDecimal = errors.New("invalid decimal")
var ErrPrecision = errors.New("decimal exceeds CPython context.prec=28")

const (
	pythonDecimalPrecision = 28
	maxRetainedScale       = 80
	mpdMinEtiny            = "-1999999999999999997"
	mpdMaxEmax             = "999999999999999999"
)

// Decimal stores an exact decimal as an integer coefficient and a decimal scale.
type Decimal struct {
	coefficient       *big.Int
	scale             int
	negativeZero      bool
	residualDirection int8
	precisionOverflow bool
}

type Mark struct {
	Value       Decimal
	NaN         bool
	NegativeNaN bool
	NaNPayload  *big.Int
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
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "nan") {
		payloadText := text[3:]
		if payloadText == "" {
			return Mark{NaN: true, NegativeNaN: sign == '-'}, nil
		}
		if !validDigitSeparators(payloadText) {
			return Mark{}, ErrInvalidDecimal
		}
		payload, ok := new(big.Int).SetString(strings.ReplaceAll(payloadText, "_", ""), 10)
		if !ok {
			return Mark{}, ErrInvalidDecimal
		}
		payload.Mod(payload, tenTo(pythonDecimalPrecision))
		return Mark{NaN: true, NegativeNaN: sign == '-', NaNPayload: payload}, nil
	}
	switch lower {
	case "snan", "inf", "infinity":
		return Mark{}, ErrInvalidDecimal
	default:
		value, err := Parse(stringWithSign(sign, text))
		return Mark{Value: value}, err
	}
}

func (m Mark) NaNString() string {
	if !m.NaN {
		return ""
	}
	text := "NaN"
	if m.NaNPayload != nil && m.NaNPayload.Sign() != 0 {
		text += m.NaNPayload.String()
	}
	if m.NegativeNaN {
		return "-" + text
	}
	return text
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
	exponent := big.NewInt(0)
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
	if !strings.ContainsAny(strings.ReplaceAll(text, "_", ""), "0123456789") {
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
	trueExponent := new(big.Int).Neg(big.NewInt(int64(len(fraction))))
	trueExponent.Add(trueExponent, exponent)
	adjustedExponent := new(big.Int).Add(trueExponent, big.NewInt(int64(len(digits)-1)))
	minEtiny, _ := new(big.Int).SetString(mpdMinEtiny, 10)
	maxEmax, _ := new(big.Int).SetString(mpdMaxEmax, 10)
	if trueExponent.Cmp(minEtiny) < 0 || adjustedExponent.Cmp(maxEmax) > 0 {
		return Decimal{}, ErrInvalidDecimal
	}
	scale := 0
	if trueExponent.Sign() > 0 {
		if trueExponent.Cmp(big.NewInt(60)) <= 0 {
			scale = int(trueExponent.Int64())
			coefficient.Mul(coefficient, tenTo(scale))
			scale = 0
		} else {
			return Decimal{
				coefficient:       coefficient,
				negativeZero:      negative && coefficient.Sign() == 0,
				precisionOverflow: coefficient.Sign() != 0,
			}, nil
		}
	} else {
		scaleBig := new(big.Int).Neg(trueExponent)
		if scaleBig.IsInt64() &&
			scaleBig.Int64() <= int64(maxRetainedScale+len(digits)) {
			scale = int(scaleBig.Int64())
		} else {
			scale = maxRetainedScale + len(digits)
		}
	}
	if scale > maxRetainedScale {
		drop := scale - maxRetainedScale
		nonzero := coefficient.Sign() != 0
		direction := int8(0)
		if nonzero {
			direction = residualSign(coefficient, negative)
		}
		digitLength := len(new(big.Int).Abs(coefficient).String())
		if drop >= digitLength {
			coefficient.SetInt64(0)
		} else {
			divisor := tenTo(drop)
			quotient, remainder := new(big.Int).QuoRem(coefficient, divisor, new(big.Int))
			coefficient = quotient
			if remainder.Sign() == 0 {
				direction = 0
			}
		}
		scale = maxRetainedScale
		return Decimal{
			coefficient:       coefficient,
			scale:             scale,
			negativeZero:      negative && coefficient.Sign() == 0,
			residualDirection: direction,
		}, nil
	}
	return Decimal{
		coefficient:  coefficient,
		scale:        scale,
		negativeZero: negative && coefficient.Sign() == 0,
	}, nil
}

func stringWithSign(sign byte, value string) string {
	if sign == 0 {
		return value
	}
	return string(sign) + value
}

func (d Decimal) Add(other Decimal) Decimal {
	if d.precisionOverflow || other.precisionOverflow {
		return Decimal{coefficient: big.NewInt(0), precisionOverflow: true}
	}
	scale := d.scale
	if other.scale > scale {
		scale = other.scale
	}
	left := scaledCoefficient(d, scale)
	right := scaledCoefficient(other, scale)
	left.Add(left, right)
	return roundToPrecision(Decimal{
		coefficient:       left,
		scale:             scale,
		negativeZero:      left.Sign() == 0 && d.negativeZero && other.negativeZero,
		residualDirection: combineResidual(d.residualDirection, other.residualDirection),
		precisionOverflow: d.precisionOverflow || other.precisionOverflow,
	})
}

func (d Decimal) Sub(other Decimal) Decimal {
	if d.precisionOverflow || other.precisionOverflow {
		return Decimal{coefficient: big.NewInt(0), precisionOverflow: true}
	}
	scale := d.scale
	if other.scale > scale {
		scale = other.scale
	}
	left := scaledCoefficient(d, scale)
	right := scaledCoefficient(other, scale)
	left.Sub(left, right)
	return roundToPrecision(Decimal{
		coefficient:       left,
		scale:             scale,
		negativeZero:      left.Sign() == 0 && d.negativeZero && other.coefficient.Sign() == 0 && !other.negativeZero,
		residualDirection: combineResidual(d.residualDirection, -other.residualDirection),
		precisionOverflow: d.precisionOverflow || other.precisionOverflow,
	})
}

func (d Decimal) MulInt64(value int64) Decimal {
	coefficient := new(big.Int).Mul(d.coefficient, big.NewInt(value))
	negative := d.coefficient.Sign() < 0 || d.negativeZero
	return roundToPrecision(Decimal{
		coefficient:       coefficient,
		scale:             d.scale,
		negativeZero:      coefficient.Sign() == 0 && negative != (value < 0),
		residualDirection: multiplyResidual(d.residualDirection, value),
		precisionOverflow: d.precisionOverflow && value != 0,
	})
}

func (d Decimal) Quantize(scale int) (Decimal, error) {
	if d.precisionOverflow {
		return Decimal{}, ErrPrecision
	}
	var value Decimal
	if scale >= d.scale {
		coefficient := new(big.Int).Mul(d.coefficient, tenTo(scale-d.scale))
		value = Decimal{
			coefficient:       coefficient,
			scale:             scale,
			negativeZero:      d.negativeZero,
			residualDirection: d.residualDirection,
			precisionOverflow: d.precisionOverflow,
		}
		return checkPrecision(value)
	}
	return roundRatio(d.coefficient, tenTo(d.scale-scale), scale, d.residualDirection, d.negativeZero)
}

// DivideQuantized divides d by divisor and reports context precision errors.
func (d Decimal) DivideQuantized(divisor int64, scale int) (Decimal, error) {
	if divisor == 0 {
		panic("division by zero")
	}
	if d.precisionOverflow {
		return Decimal{}, ErrPrecision
	}
	value := divideToPrecision(d, divisor)
	return value.Quantize(scale)
}

func (d Decimal) StringFixed(scale int) (string, error) {
	quantized, err := d.Quantize(scale)
	if err != nil {
		return "", err
	}
	negative := quantized.coefficient.Sign() < 0 ||
		(quantized.coefficient.Sign() == 0 && quantized.negativeZero)
	absolute := new(big.Int).Abs(quantized.coefficient)
	digits := absolute.String()
	if scale == 0 {
		if negative {
			return "-" + digits, nil
		}
		return digits, nil
	}
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale+1-len(digits)) + digits
	}
	result := digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
	if negative {
		return "-" + result, nil
	}
	return result, nil
}

func (d Decimal) String() string {
	value, err := d.StringFixed(d.scale)
	if err == nil {
		return value
	}
	// String is for debugging; preserve the exact coefficient and scale if
	// formatting would exceed the Python-compatible precision ceiling.
	return d.coefficient.String() + "e-" + strconv.Itoa(d.scale)
}

func checkPrecision(value Decimal) (Decimal, error) {
	if value.precisionOverflow {
		return Decimal{}, ErrPrecision
	}
	if len(new(big.Int).Abs(value.coefficient).String()) > pythonDecimalPrecision {
		return Decimal{}, ErrPrecision
	}
	return value, nil
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

func parseExponent(value string) (*big.Int, bool) {
	if value == "" {
		return nil, false
	}
	sign := 1
	if value[0] == '+' || value[0] == '-' {
		if value[0] == '-' {
			sign = -1
		}
		value = value[1:]
	}
	if !validDigitSeparators(value) {
		return nil, false
	}
	value = strings.ReplaceAll(value, "_", "")
	exponent := new(big.Int)
	if _, ok := exponent.SetString(value, 10); !ok {
		return nil, false
	}
	if sign < 0 {
		exponent.Neg(exponent)
	}
	return exponent, true
}

func scaledCoefficient(value Decimal, scale int) *big.Int {
	return new(big.Int).Mul(value.coefficient, tenTo(scale-value.scale))
}

func tenTo(scale int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
}

func roundRatio(numerator, denominator *big.Int, scale int, residualDirection int8, negativeZero bool) (Decimal, error) {
	return checkPrecision(roundRatioRaw(numerator, denominator, scale, residualDirection, negativeZero))
}

func roundRatioRaw(numerator, denominator *big.Int, scale int, residualDirection int8, negativeZero bool) Decimal {
	if denominator.Sign() == 0 {
		panic("division by zero")
	}
	negative := numerator.Sign() < 0
	absoluteNumerator := new(big.Int).Abs(numerator)
	absoluteDenominator := new(big.Int).Abs(denominator)
	quotient, remainder := new(big.Int).QuoRem(absoluteNumerator, absoluteDenominator, new(big.Int))
	twiceRemainder := new(big.Int).Lsh(remainder, 1)
	roundUp := twiceRemainder.Cmp(absoluteDenominator) > 0
	if twiceRemainder.Cmp(absoluteDenominator) == 0 {
		switch {
		case residualDirection == signOf(negative):
			roundUp = true
		case residualDirection == -signOf(negative):
			roundUp = false
		default:
			roundUp = quotient.Bit(0) == 1
		}
	}
	if roundUp {
		quotient.Add(quotient, big.NewInt(1))
	}
	if negative {
		quotient.Neg(quotient)
	}
	return Decimal{
		coefficient:  quotient,
		scale:        scale,
		negativeZero: (negative && quotient.Sign() == 0) || (quotient.Sign() == 0 && negativeZero),
	}
}

func roundToPrecision(value Decimal) Decimal {
	if value.precisionOverflow {
		return value
	}
	if value.coefficient.Sign() == 0 {
		value.residualDirection = 0
		return value
	}
	digits := len(new(big.Int).Abs(value.coefficient).String())
	if digits <= pythonDecimalPrecision {
		value.residualDirection = 0
		return value
	}
	targetScale := value.scale - (digits - pythonDecimalPrecision)
	rounded := roundRatioRaw(
		value.coefficient,
		tenTo(digits-pythonDecimalPrecision),
		targetScale,
		value.residualDirection,
		value.negativeZero,
	)
	rounded.residualDirection = 0
	if rounded.scale < 0 {
		rounded.coefficient.Mul(rounded.coefficient, tenTo(-rounded.scale))
		rounded.scale = 0
	}
	return rounded
}

func divideToPrecision(value Decimal, divisor int64) Decimal {
	if value.precisionOverflow {
		return value
	}
	if value.coefficient.Sign() == 0 {
		return Decimal{coefficient: big.NewInt(0), negativeZero: value.negativeZero}
	}
	numerator := new(big.Int).Abs(value.coefficient)
	denominator := new(big.Int).Abs(big.NewInt(divisor))
	digitsNumerator := len(numerator.String())
	digitsDenominator := len(denominator.String())
	delta := digitsNumerator - digitsDenominator
	adjustedExponent := delta - value.scale
	if delta >= 0 {
		if numerator.Cmp(new(big.Int).Mul(denominator, tenTo(delta))) < 0 {
			adjustedExponent--
		}
	} else if new(big.Int).Mul(numerator, tenTo(-delta)).Cmp(denominator) < 0 {
		adjustedExponent--
	}
	targetScale := pythonDecimalPrecision - adjustedExponent - 1
	if targetScale > maxRetainedScale {
		negative := value.coefficient.Sign() < 0
		if divisor < 0 {
			negative = !negative
		}
		return Decimal{
			coefficient:  big.NewInt(0),
			scale:        maxRetainedScale,
			negativeZero: value.negativeZero || negative,
		}
	}
	denominator.Mul(denominator, tenTo(value.scale))
	if targetScale >= 0 {
		numerator.Mul(numerator, tenTo(targetScale))
	} else {
		denominator.Mul(denominator, tenTo(-targetScale))
	}
	if value.coefficient.Sign() < 0 {
		numerator.Neg(numerator)
	}
	if divisor < 0 {
		numerator.Neg(numerator)
	}
	rounded := roundRatioRaw(numerator, denominator, targetScale, value.residualDirection, value.negativeZero)
	rounded.residualDirection = 0
	if rounded.scale < 0 {
		rounded.coefficient.Mul(rounded.coefficient, tenTo(-rounded.scale))
		rounded.scale = 0
	}
	return rounded
}

func residualSign(coefficient *big.Int, negative bool) int8 {
	if coefficient.Sign() == 0 {
		if negative {
			return -1
		}
		return 1
	}
	if coefficient.Sign() < 0 {
		return -1
	}
	return 1
}

func signOf(negative bool) int8 {
	if negative {
		return -1
	}
	return 1
}

func multiplyResidual(direction int8, value int64) int8 {
	if direction == 0 || value == 0 {
		return 0
	}
	if value < 0 {
		return -direction
	}
	return direction
}

func combineResidual(left, right int8) int8 {
	if left == 0 {
		return right
	}
	if right == 0 || left == right {
		return left
	}
	// Opposing inexact operands are unreachable for portfolio arithmetic:
	// stored lot prices have at most six fractional digits and only marks
	// originate outside that bound.
	return 0
}
