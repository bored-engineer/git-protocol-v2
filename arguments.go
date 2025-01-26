package protocolv2

import (
	"bytes"
	"fmt"
	"strings"

	pktline "github.com/bored-engineer/git-pkt-line"
)

// command-specific-args are packet line framed arguments defined by each individual command.
type CommandArgument struct {
	Key   string
	Value string
}

// Append the argument pkt-line to the given slice
func (ca CommandArgument) Append(b []byte) []byte {
	if len(ca.Value) == 0 {
		return pktline.AppendString(b, ca.Key)
	}
	return pktline.AppendString(b, ca.Key+" "+ca.Value)
}

// Bytes returns the argument pkt-line as a slice
func (ca CommandArgument) Bytes() []byte {
	return ca.Append(nil)
}

// String implements the fmt.Stringer interface
func (ca CommandArgument) String() string {
	if len(ca.Value) == 0 {
		return ca.Key
	}
	return ca.Key + " " + ca.Value
}

// Parse the values from a given pkt-line
func (ca *CommandArgument) Parse(line []byte) error {
	key, value, ok := bytes.Cut(line, []byte(" "))
	if len(key) == 0 {
		return fmt.Errorf("invalid argument: %q", string(line))
	}
	ca.Key = string(key)
	if ok {
		if len(value) == 0 {
			return fmt.Errorf("invalid argument: %q", string(line))
		}
		ca.Value = string(value)
	}
	return nil
}

// command-args = *command-specific-arg
type CommandArguments []CommandArgument

// Append the argument pkt-lines to the given slice
func (cas CommandArguments) Append(b []byte) []byte {
	for _, a := range cas {
		b = a.Append(b)
	}
	return b
}

// Bytes returns the capability pkt-lines as a slice
func (cas CommandArguments) Bytes() []byte {
	return cas.Append(nil)
}

// String implements the fmt.Stringer interface
func (cas CommandArguments) String() string {
	var sb strings.Builder
	sb.WriteRune('<')
	for idx, cap := range cas {
		if idx != 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(cap.String())
	}
	sb.WriteRune('>')
	return sb.String()
}

// Get returns the value of the given key in the capabilities
func (cas CommandArguments) Get(key string) (value string, ok bool) {
	for _, arg := range cas {
		if arg.Key == key {
			return arg.Value, true
		}
	}
	return "", false
}

// Has returns true if the given key is present in the capabilities
func (cas CommandArguments) Has(key string) bool {
	for _, arg := range cas {
		if arg.Key == key {
			return true
		}
	}
	return false
}

// Parse consumes the arguments from a given pkt-line scanner
func (cas *CommandArguments) Parse(scanner *pktline.Scanner) error {
	for {
		line, err := scanner.Scan()
		if err != nil {
			return err
		}
		var ca CommandArgument
		if err := ca.Parse(line); err != nil {
			return err
		}
		*cas = append(*cas, ca)
	}
}
