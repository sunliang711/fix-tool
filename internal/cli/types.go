package cli

import "io"

type Args []string

type IO struct {
	Out    io.Writer
	ErrOut io.Writer
}
