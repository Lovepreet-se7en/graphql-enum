package utils

import "strings"

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ContainsCaseInsensitive(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
