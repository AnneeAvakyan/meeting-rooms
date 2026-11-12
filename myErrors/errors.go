package myErrors

import "errors"

var ErrNotFound = errors.New("not found")
var ErrOverlap = errors.New("service overlaps with an existing one")
var ErrInvalidInterval = errors.New("start_time must be before end_time")
