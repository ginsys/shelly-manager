package shelly

import "math"

// maxExactInteger is the largest whole number a float64 holds with no neighbour: 2^53-1. A JSON number
// above it has already lost digits on its way into a float64, so it is not read as the number it was.
const maxExactInteger = 1<<53 - 1

// WholeNumber reads a decoded JSON number as an int, and says false for anything else. A fraction, a
// value no int holds exactly, NaN, an infinity and any type but float64 read as no value: a fraction is
// never cut to its whole part, so a stored 7 cannot stand for a device's 7.5 and a comparison shows the
// difference. Every read of auto_on, auto_off and max_power goes through it.
func WholeNumber(raw interface{}) (int, bool) {
	n, ok := raw.(float64)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || math.Abs(n) > maxExactInteger {
		return 0, false
	}
	return int(n), true
}
