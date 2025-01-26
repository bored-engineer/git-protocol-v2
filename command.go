package protocolv2

import (
	"bytes"
	"errors"
	"fmt"

	pktline "github.com/bored-engineer/git-pkt-line"
)

// https://git-scm.com/docs/protocol-v2#_command_request
type CommandRequest struct {
	Command      string
	Capabilities Capabilities
	Arguments    CommandArguments
}

// Bytes returns the command-request pkt-lines to the given slice
func (cr CommandRequest) Append(b []byte) []byte {
	b = pktline.AppendString(b, "command="+cr.Command+"\n")
	b = cr.Capabilities.Append(b)
	b = pktline.AppendDelimPkt(b)
	for _, arg := range cr.Arguments {
		b = arg.Append(b)
	}
	b = pktline.AppendFlushPkt(b)
	return b
}

// Bytes returns the command-request pkt-lines as a slice
func (cr CommandRequest) Bytes() []byte {
	return cr.Append(nil)
}

// Parse populates the fields from a given pkt-line scanner
func (cr *CommandRequest) Parse(scanner *pktline.Scanner) error {
	line, err := scanner.Scan()
	if err != nil {
		return err
	}
	remaining, ok := bytes.CutSuffix(line, []byte("\n"))
	if !ok {
		return fmt.Errorf("invalid command-request: %q", string(line))
	}
	command, ok := bytes.CutPrefix(remaining, []byte("command="))
	if !ok {
		return fmt.Errorf("invalid command-request: %q", string(line))
	}
	cr.Command = string(command)
	if err := cr.Capabilities.Parse(scanner); err != nil && !errors.Is(err, pktline.ErrDelimPkt) {
		return err
	}
	if err := cr.Arguments.Parse(scanner); err != nil && !errors.Is(err, pktline.ErrFlushPkt) {
		return err
	}
	return nil
}
