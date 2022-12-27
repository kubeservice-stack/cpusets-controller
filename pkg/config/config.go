package config

import (
	"os"
)

var (
	NodeName  string
	FileMatch string
)

func init() {
	NodeName = os.Getenv("NODE_NAME")
	FileMatch = os.Getenv("FILE_MATCH")
}
