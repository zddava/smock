package build

import "fmt"

var (
	Module  string
	Version string
	Date    string
)

func ToString() string {
	return fmt.Sprintf("Module: %s\nVersion: %s\nDate: %s", Module, Version, Date)
}
