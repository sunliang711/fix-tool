package cli

import "io"

type Args []string

type IO struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}
