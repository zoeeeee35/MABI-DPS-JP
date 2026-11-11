package main

import "fmt"

func formatNum(n float64) string {
	whole := fmt.Sprintf("%.0f", n)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	return whole
}
