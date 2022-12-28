package utils

import (
	"github.com/mcuadros/go-version"
)

// Usage
//     Utils.CompareSimple("1.2", "1.0.1")
//     Returns: 1
//
//     Utils.CompareSimple("1.0rc1", "1.0")
//     Returns: -1
func Version_compare(version1, version2 string) int {
	return version.CompareSimple(version1, version2)
}
