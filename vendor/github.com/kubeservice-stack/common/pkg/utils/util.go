package utils

func Mix(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a <= b {
		return b
	}
	return a
}

func MinFloat64(a, b float64) float64 {
	if a <= b {
		return a
	}
	return b
}

func MaxFloat64(a, b float64) float64 {
	if a <= b {
		return b
	}
	return a
}
