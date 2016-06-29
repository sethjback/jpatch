package jpatch

import "strconv"

func ValidArrayIndex(in interface{}) bool {
	_, ok := in.(int)
	if ok {
		return true
	}

	st, ok := in.(string)
	if ok {
		if st == "-" {
			return true
		}
		if _, err := strconv.Atoi(st); err == nil {
			return true
		}
	}

	return false
}
