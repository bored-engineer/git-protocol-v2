package protocolv2

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	pktline "github.com/bored-engineer/git-pkt-line"
)

const (
	// In addition to the object pointed by it, show the underlying ref
	// pointed by it when showing a symbolic ref.
	ArgumentSymRefs = "symrefs"
	// Show peeled tags.
	ArgumentPeel = "peel"
	// When specified, only references having a prefix matching one of
	// the provided prefixes are displayed. Multiple instances may be
	// given, in which case references matching any prefix will be
	// shown. Note that this is purely for optimization; a server MAY
	// show refs not matching the prefix if it chooses, and clients
	// should filter the result themselves.
	ArgumentRefPrefix = "ref-prefix"
	// The server will send information about HEAD even if it is a symref
	// pointing to an unborn branch in the form "unborn HEAD
	// symref-target:<target>".
	ArgumentUnborn = "unborn"
)

// ref = PKT-LINE(obj-id-or-unborn SP refname *(SP ref-attribute) LF)
type Reference struct {
	// obj-id-or-unborn = (obj-id | "unborn")
	ObjectID string
	Name     string
	// ref-attribute = (symref | peeled)
	Attributes []string
}

// Append the reference pkt-line to the given slice
func (r Reference) Append(b []byte) []byte {
	sz := len(r.ObjectID) + len(" ") + len(r.Name)
	for _, attr := range r.Attributes {
		sz += len(" ") + len(attr)
	}
	sz += len("\n")
	b = pktline.AppendLength(b, sz)
	b = append(b, r.ObjectID...)
	b = append(b, ' ')
	b = append(b, r.Name...)
	for _, attr := range r.Attributes {
		b = append(b, ' ')
		b = append(b, attr...)
	}
	b = append(b, '\n')
	return b
}

// Bytes returns the reference pkt-line as a slice
func (r Reference) Bytes() []byte {
	return r.Append(nil)
}

// String implements the fmt.Stringer interface
func (r Reference) String() string {
	var sb strings.Builder
	sb.WriteString(r.ObjectID)
	sb.WriteByte(' ')
	sb.WriteString(r.Name)
	for _, attr := range r.Attributes {
		sb.WriteByte(' ')
		sb.WriteString(attr)
	}
	return sb.String()
}

// Parse populates the fields from a given pkt-line slice
func (r *Reference) Parse(line []byte) error {
	remaining, ok := bytes.CutSuffix(line, []byte("\n"))
	if !ok {
		return fmt.Errorf("invalid ref: %q", string(line))
	}
	objID, remaining, ok := bytes.Cut(remaining, []byte(" "))
	if !ok {
		return fmt.Errorf("invalid ref: %q", string(line))
	}
	r.ObjectID = string(objID)
	name, remaining, ok := bytes.Cut(remaining, []byte(" "))
	r.Name = string(name)
	for ok {
		var attr []byte
		attr, remaining, ok = bytes.Cut(remaining, []byte(" "))
		r.Attributes = append(r.Attributes, string(attr))
	}
	return nil
}

// https://git-scm.com/docs/protocol-v2#_ls_refs
type ListReferencesResponse struct {
	References []Reference
}

// Append the response pkt-line to the given slice
func (lrs ListReferencesResponse) Append(b []byte) []byte {
	for _, ref := range lrs.References {
		b = ref.Append(b)
	}
	b = pktline.AppendFlushPkt(b)
	return b
}

// Bytes returns the response pkt-line as a slice
func (lrs ListReferencesResponse) Bytes() []byte {
	return lrs.Append(nil)
}

// Map converts the slice into a map of reference names to object ID
func (lrs ListReferencesResponse) Map() map[string]string {
	m := make(map[string]string, len(lrs.References))
	for _, ref := range lrs.References {
		m[ref.Name] = ref.ObjectID
	}
	return m
}

// Parse populates the fields from a given pkt-line scanner
func (lrs *ListReferencesResponse) Parse(scanner *pktline.Scanner) error {
	for {
		line, err := scanner.Scan()
		if err != nil {
			if errors.Is(err, pktline.ErrFlushPkt) {
				return nil
			}
			return err
		}
		var ref Reference
		if err := ref.Parse(line); err != nil {
			return err
		}
		lrs.References = append(lrs.References, ref)
	}
}
