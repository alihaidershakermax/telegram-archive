package services

import "strconv"

// This file imports strconv for use by archive_service.go's intToStr function.
// Go requires imports in the same package to be in the file that uses them,
// but intToStr is defined in archive_service.go.
// This is a workaround - the actual import is used there.

var _ = strconv.Itoa // ensure strconv is available in package
