package lb

import "errors"

var errTS = errors.New("bad timestamp")

// ParseTimestamp uses the same parser in both languages for a fair comparison.
func ParseTimestamp(s string) (int64, error) {
	if len(s) != 20 || s[4] != '-' || s[7] != '-' || s[10] != 'T' || s[13] != ':' || s[16] != ':' || s[19] != 'Z' {
		return 0, errTS
	}
	num := func(a, b int) (int64, error) {
		var v int64
		for i := a; i < b; i++ {
			c := s[i]
			if c < '0' || c > '9' {
				return 0, errTS
			}
			v = v*10 + int64(c-'0')
		}
		return v, nil
	}
	y, e1 := num(0, 4)
	mo, e2 := num(5, 7)
	d, e3 := num(8, 10)
	h, e4 := num(11, 13)
	mi, e5 := num(14, 16)
	se, e6 := num(17, 19)
	if err := errors.Join(e1, e2, e3, e4, e5, e6); err != nil {
		return 0, err
	}
	return daysFromCivil(y, mo, d)*86400 + h*3600 + mi*60 + se, nil
}

// daysFromCivil is Howard Hinnant's algorithm.
func daysFromCivil(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 && y%400 != 0 {
		era--
	}
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}
