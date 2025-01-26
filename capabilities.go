package protocolv2

import (
	"bytes"
	"fmt"
	"strings"

	pktline "github.com/bored-engineer/git-pkt-line"
)

const (
	// The server can advertise the `agent` capability with a value `X` (in the
	// form `agent=X`) to notify the client that the server is running version
	// `X`.  The client may optionally send its own agent string by including
	// the `agent` capability with a value `Y` (in the form `agent=Y`) in its
	// request to the server (but it MUST NOT do so if the server did not
	// advertise the agent capability). The `X` and `Y` strings may contain any
	// printable ASCII characters except space (i.e., the byte range 32 < x <
	// 127), and are typically of the form "package/version" (e.g.,
	// "git/1.8.3.1"). The agent strings are purely informative for statistics
	// and debugging purposes, and MUST NOT be used to programmatically assume
	// the presence or absence of particular features.
	CapabilityAgent = "agent"
	// If advertised, indicates that any number of server specific options can be
	// included in a request.  This is done by sending each option as a
	// "server-option=<option>" capability line in the capability-list section of
	// a request.
	CapabilityServerOption = "server-option"
	// The server can advertise the `object-format` capability with a value `X` (in the
	// form `object-format=X`) to notify the client that the server is able to deal
	// with objects using hash algorithm X.  If not specified, the server is assumed to
	// only handle SHA-1.  If the client would like to use a hash algorithm other than
	// SHA-1, it should specify its object-format string.
	CapabilityObjectFormat = "object-format"
	// The server may advertise a session ID that can be used to identify this process
	// across multiple requests. The client may advertise its own session ID back to
	// the server as well.
	//
	// Session IDs should be unique to a given process. They must fit within a
	// packet-line, and must not contain non-printable or whitespace characters. The
	// current implementation uses trace2 session IDs (see
	// link:technical/api-trace2.html[api-trace2] for details), but this may change
	// and users of the session ID should not rely on this fact.
	CapabilitySessionID = "session-id"
	// ls-refs is the command used to request a reference advertisement in v2. Unlike
	// the current reference advertisement, ls-refs takes in arguments which can be
	// used to limit the refs sent from the server.
	CapabilityListReferences = "ls-refs"
	// fetch is the command used to fetch a packfile in v2. It can be looked at as a
	// modified version of the v1 fetch where the ref-advertisement is stripped out
	// (since the ls-refs command fills that role) and the message format is tweaked
	// to eliminate redundancies and permit easy addition of future extensions.
	CapabilityFetch = "fetch"
	// object-info is the command to retrieve information about one or more objects.
	// Its main purpose is to allow a client to make decisions based on this
	// information without having to fully fetch objects. Object size is the only
	// information that is currently supported.
	CapabilityObjectInfo = "object-info"
)

// capability = PKT-LINE(key[=value] LF)
type Capability struct {
	// key = 1*(ALPHA | DIGIT | "-_")
	Key string
	// value = 1*(ALPHA | DIGIT | " -_.,?\/{}[]()<>!@#$%^&*+=:;")
	Value string
}

// Append the capability pkt-line to the given slice
func (c Capability) Append(b []byte) []byte {
	if len(c.Value) > 0 {
		b = pktline.AppendLength(b, len(c.Key)+1+len(c.Value)+1)
		b = append(b, c.Key...)
		b = append(b, '=')
		b = append(b, c.Value...)
		b = append(b, '\n')
	} else {
		b = pktline.AppendLength(b, len(c.Key)+1)
		b = append(b, c.Key...)
		b = append(b, '\n')
	}
	return b
}

// Bytes returns the capability pkt-line as a slice
func (c Capability) Bytes() []byte {
	return c.Append(nil)
}

// String implements the fmt.Stringer interface
func (c Capability) String() string {
	if len(c.Value) > 0 {
		return c.Key + "=" + c.Value
	} else {
		return c.Key
	}
}

// Parse the capability from a given pkt-line
func (c *Capability) Parse(line []byte) error {
	remaining, ok := bytes.CutSuffix(line, []byte("\n"))
	if !ok {
		return fmt.Errorf("invalid capability: %q", string(line))
	}
	key, value, ok := bytes.Cut(remaining, []byte("="))
	if len(key) == 0 {
		return fmt.Errorf("invalid capability: %q", string(line))
	}
	c.Key = string(key)
	if ok {
		if len(value) == 0 {
			return fmt.Errorf("invalid capability: %q", string(line))
		}
		c.Value = string(value)
	}
	return nil
}

// capability-list = *capability
type Capabilities []Capability

// Append the capability pkt-lines to the given slice
func (cs Capabilities) Append(b []byte) []byte {
	for _, cap := range cs {
		b = cap.Append(b)
	}
	return b
}

// Bytes returns the capability pkt-lines as a slice
func (cs Capabilities) Bytes() []byte {
	return cs.Append(nil)
}

// String implements the fmt.Stringer interface
func (cs Capabilities) String() string {
	var sb strings.Builder
	for idx, cap := range cs {
		if idx != 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(cap.String())
	}
	return sb.String()
}

// Get returns the value of the given key in the capabilities
func (cc Capabilities) Get(key string) (value string, ok bool) {
	for _, c := range cc {
		if c.Key == key {
			return c.Value, true
		}
	}
	return "", false
}

// Has returns true if the given key is present in the capabilities
func (cs Capabilities) Has(key string) bool {
	for _, c := range cs {
		if c.Key == key {
			return true
		}
	}
	return false
}

// Parse the capabilities from a given pkt-line
func (cs *Capabilities) Parse(scanner *pktline.Scanner) error {
	for {
		line, err := scanner.Scan()
		if err != nil {
			return err
		}
		var cap Capability
		if err := cap.Parse(line); err != nil {
			return err
		}
		*cs = append(*cs, cap)
	}
}
