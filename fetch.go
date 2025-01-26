package protocolv2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"

	pktline "github.com/bored-engineer/git-pkt-line"
)

const (
	// Indicates to the server an object which the client wants to
	// retrieve.  Wants can be anything and are not limited to
	// advertised objects.
	ArgumentWant = "want"
	// Indicates to the server an object which the client has locally.
	// This allows the server to make a packfile which only contains
	// the objects that the client needs. Multiple 'have' lines can be
	// supplied.
	ArgumentHave = "have"
	// Indicates to the server that negotiation should terminate (or
	// not even begin if performing a clone) and that the server should
	// use the information supplied in the request to construct the
	// packfile.
	ArgumentDone = "done"
	// Request that a thin pack be sent, which is a pack with deltas
	// which reference base objects not contained within the pack (but
	// 	are known to exist at the receiving end). This can reduce the
	// 	network traffic significantly, but it requires the receiving end
	// 	to know how to "thicken" these packs by adding the missing bases
	// 	to the pack.
	ArgumentThinPack = "thin-pack"
	// Request that progress information that would normally be sent on
	// side-band channel 2, during the packfile transfer, should not be
	// sent.  However, the side-band channel 3 is still used for error
	// responses.
	ArgumentNoProgress = "no-progress"
	// Request that annotated tags should be sent if the objects they
	// point to are being sent.
	ArgumentIncludeTag = "include-tag"
	// Indicate that the client understands PACKv2 with delta referring
	// to its base by position in pack rather than by an oid.  That is,
	// they can read OBJ_OFS_DELTA (aka type 6) in a packfile.
	ArgumentOFSDelta = "ofs-delta"
	// A client must notify the server of all commits for which it only
	// has shallow copies (meaning that it doesn't have the parents of
	// a commit) by supplying a 'shallow <oid>' line for each such
	// object so that the server is aware of the limitations of the
	// client's history.  This is so that the server is aware that the
	// client may not have all objects reachable from such commits.
	ArgumentShallow = "shallow"
	// Requests that the fetch/clone should be shallow having a commit
	// depth of <depth> relative to the remote side.
	ArgumentDeepen = "deepen"
	// Requests that the semantics of the "deepen" command be changed
	// to indicate that the depth requested is relative to the client's
	// current shallow boundary, instead of relative to the requested
	// commits.
	ArgumentDeepenRelative = "deepen-relative"
	// Requests that the shallow clone/fetch should be cut at a
	// specific time, instead of depth.  Internally it's equivalent to
	// doing "git rev-list --max-age=<timestamp>". Cannot be used with
	// "deepen".
	ArgumentDeepenSince = "deepen-since"
	// Requests that the shallow clone/fetch should be cut at a
	// specific revision specified by '<rev>', instead of a depth.
	// Internally it's equivalent of doing "git rev-list --not <rev>".
	// Cannot be used with "deepen", but can be used with
	// "deepen-since".
	ArgumentDeepenNot = "deepen-not"
	// Request that various objects from the packfile be omitted
	// using one of several filtering techniques. These are intended
	// for use with partial clone and partial fetch operations. See
	// `rev-list` for possible "filter-spec" values. When communicating
	// with other processes, senders SHOULD translate scaled integers
	// (e.g. "1k") into a fully-expanded form (e.g. "1024") to aid
	// interoperability with older receivers that may not understand
	// newly-invented scaling suffixes. However, receivers SHOULD
	// accept the following suffixes: 'k', 'm', and 'g' for 1024,
	// 1048576, and 1073741824, respectively.
	ArgumentFilter = "filter"
	// Indicates to the server that the client wants to retrieve a
	// particular ref, where <ref> is the full name of a ref on the
	// server.
	ArgumentWantRef = "want-ref"
	// Instruct the server to send the whole response multiplexed, not just
	// the packfile section. All non-flush and non-delim PKT-LINE in the
	// response (not only in the packfile section) will then start with a byte
	// indicating its sideband (1, 2, or 3), and the server may send "0005\2"
	// (a PKT-LINE of sideband 2 with no payload) as a keepalive packet.
	ArgumentSidebandAll = "sideband-all"
	// Indicates to the server that the client is willing to receive
	// URIs of any of the given protocols in place of objects in the
	// sent packfile. Before performing the connectivity check, the
	// client should download from all given URIs. Currently, the
	// protocols supported are "http" and "https".
	ArgumentPackfileURIs = "packfile-uris"
	// Indicates to the server that it should never send "ready", but
	// should wait for the client to say "done" before sending the
	// packfile.
	ArgumentWaitForDone = "wait-for-done"
)

// acknowledgments = PKT-LINE("acknowledgments" LF) (nak | *ack) (ready)
// ready = PKT-LINE("ready" LF)
// nak = PKT-LINE("NAK" LF)
// ack = PKT-LINE("ACK" SP obj-id LF)
type Acknowledgements struct {
	Ready bool
	NAK   bool
	ACKs  []string
}

// IsZero returns true if the struct matches the zero value
func (a Acknowledgements) IsZero() bool {
	return !a.Ready && !a.NAK && a.ACKs == nil
}

// Append the response pkt-line to the given slice
func (a Acknowledgements) Append(b []byte) []byte {
	b = pktline.AppendString(b, "acknowledgments\n")
	if a.Ready {
		b = pktline.AppendString(b, "ready\n")
	}
	if a.NAK {
		b = pktline.AppendString(b, "NAK\n")
	}
	for _, objID := range a.ACKs {
		b = pktline.AppendLength(b, len("ACK ")+len(objID)+len("\n"))
		b = append(b, "ACK "...)
		b = append(b, objID...)
		b = append(b, '\n')
	}
	return b
}

// Bytes returns the response pkt-line as a slice
func (a Acknowledgements) Bytes() []byte {
	return a.Append(nil)
}

// Parse populates the fields from a given pkt-line scanner
func (a *Acknowledgements) Parse(scanner *pktline.Scanner) error {
	for {
		line, err := scanner.Scan()
		if err != nil {
			return err
		}
		if objID, ok := bytes.CutPrefix(line, []byte("ACK ")); ok {
			objID, ok := bytes.CutSuffix(objID, []byte("\n"))
			if !ok {
				return fmt.Errorf("invalid ack: %q", string(line))
			}
			a.ACKs = append(a.ACKs, string(objID))
		} else if bytes.Equal(line, []byte("NAK\n")) {
			a.NAK = true
		} else if bytes.Equal(line, []byte("ready\n")) {
			a.Ready = true
		}
	}
}

// shallow = "shallow" SP obj-id
type Shallow struct {
	ObjectID string
}

// Append the response pkt-line to the given slice
func (s Shallow) Append(b []byte) []byte {
	b = pktline.AppendLength(b, len("shallow ")+len(s.ObjectID)+len("\n"))
	b = append(b, "shallow "...)
	b = append(b, s.ObjectID...)
	b = append(b, '\n')
	return b
}

// Bytes returns the response pkt-line as a slice
func (s Shallow) Bytes() []byte {
	return s.Append(nil)
}

// Parse populates the fields from a given pkt-line slice
func (s *Shallow) Parse(line []byte) error {
	remaining, ok := bytes.CutSuffix(line, []byte("\n"))
	if !ok {
		return fmt.Errorf("invalid shallow: %q", string(line))
	}
	objID, ok := bytes.CutPrefix(remaining, []byte("shallow "))
	if !ok {
		return fmt.Errorf("invalid shallow: %q", string(line))
	}
	s.ObjectID = string(objID)
	return nil
}

// unshallow = "unshallow" SP obj-id
type Unshallow struct {
	ObjectID string
}

// Append the response pkt-line to the given slice
func (u Unshallow) Append(b []byte) []byte {
	b = pktline.AppendLength(b, len("unshallow ")+len(u.ObjectID)+len("\n"))
	b = append(b, "unshallow "...)
	b = append(b, u.ObjectID...)
	b = append(b, '\n')
	return b
}

// Bytes returns the response pkt-line as a slice
func (u Unshallow) Bytes() []byte {
	return u.Append(nil)
}

// Parse populates the fields from a given pkt-line slice
func (s *Unshallow) Parse(line []byte) error {
	remaining, ok := bytes.CutSuffix(line, []byte("\n"))
	if !ok {
		return fmt.Errorf("invalid unshallow: %q", string(line))
	}
	objID, ok := bytes.CutPrefix(remaining, []byte("unshallow "))
	if !ok {
		return fmt.Errorf("invalid unshallow: %q", string(line))
	}
	s.ObjectID = string(objID)
	return nil
}

// shallow-info = PKT-LINE("shallow-info" LF)
// *PKT-LINE((shallow | unshallow) LF)
type ShallowInfo struct {
	Shallow   []Shallow
	Unshallow []Unshallow
}

// IsZero returns true if the struct matches the zero value
func (si ShallowInfo) IsZero() bool {
	return si.Shallow == nil && si.Unshallow == nil
}

// Appends the response pkt-lines to the given slice
func (si ShallowInfo) Append(b []byte) []byte {
	b = pktline.AppendString(b, "shallow-info\n")
	for _, s := range si.Shallow {
		b = s.Append(b)
	}
	for _, u := range si.Unshallow {
		b = u.Append(b)
	}
	return b
}

// Bytes returns the response pkt-line as a slice
func (si ShallowInfo) Bytes() []byte {
	return si.Append(nil)
}

// Parse populates the fields from a given pkt-line scanner
func (si *ShallowInfo) Parse(scanner *pktline.Scanner) error {
	for {
		line, err := scanner.Scan()
		if err != nil {
			return err
		}
		switch {
		case bytes.HasPrefix(line, []byte("shallow ")):
			var s Shallow
			if err := s.Parse(line); err != nil {
				return err
			}
			si.Shallow = append(si.Shallow, s)
		case bytes.HasPrefix(line, []byte("unshallow ")):
			var u Unshallow
			if err := u.Parse(line); err != nil {
				return err
			}
			si.Unshallow = append(si.Unshallow, u)
		default:
			return fmt.Errorf("invalid shallow-info: %q", string(line))
		}
	}
}

// wanted-ref = obj-id SP refname LF
type WantedRef struct {
	ObjectID string
	Name     string
}

// Appends the response pkt-lines to the given slice
func (wr WantedRef) Append(b []byte) []byte {
	b = pktline.AppendLength(b, len(wr.ObjectID)+len(" ")+len(wr.Name)+len("\n"))
	b = append(b, wr.ObjectID...)
	b = append(b, ' ')
	b = append(b, wr.Name...)
	b = append(b, '\n')
	return b
}

// Bytes returns the response pkt-line as a slice
func (wr WantedRef) Bytes() []byte {
	return wr.Append(nil)
}

// Parse populates the fields from a given pkt-line slice
func (wr *WantedRef) Parse(line []byte) error {
	remaining, ok := bytes.CutSuffix(line, []byte("\n"))
	if !ok {
		return fmt.Errorf("invalid wanted-ref: %q", string(line))
	}
	objID, name, ok := bytes.Cut(remaining, []byte(" "))
	if !ok {
		return fmt.Errorf("invalid wanted-ref: %q", string(line))
	}
	wr.ObjectID = string(objID)
	wr.Name = string(name)
	return nil
}

// wanted-refs = PKT-LINE("wanted-refs" LF) *PKT-LINE(wanted-ref)
type WantedRefs []WantedRef

// IsZero returns true if the slice matches the zero value
func (wrs WantedRefs) IsZero() bool {
	return wrs == nil
}

// Append the response pkt-lines to the given slice
func (wrs WantedRefs) Append(b []byte) []byte {
	b = pktline.AppendString(b, "wanted-refs\n")
	for _, wr := range wrs {
		b = wr.Append(b)
	}
	return b
}

// Bytes returns the response pkt-line as a slice
func (wrs WantedRefs) Bytes() []byte {
	return wrs.Append(nil)
}

// Parse populates the fields from a given pkt-line scanner
func (wrs *WantedRefs) Parse(scanner *pktline.Scanner) error {
	for {
		line, err := scanner.Scan()
		if err != nil {
			return err
		}
		var wr WantedRef
		if err := wr.Parse(line); err != nil {
			return err
		}
		*wrs = append(*wrs, wr)
	}
}

// packfile-uri = PKT-LINE(40*(HEXDIGIT) SP *%x20-ff LF)
type PackfileURI struct {
	Checksum string
	URI      string
}

// Append the response pkt-line to the given slice
func (pu PackfileURI) Append(b []byte) []byte {
	b = pktline.AppendLength(b, len(pu.Checksum)+len(" ")+len(pu.URI)+len("\n"))
	b = append(b, pu.Checksum...)
	b = append(b, ' ')
	b = append(b, pu.URI...)
	b = append(b, '\n')
	return b
}

// Bytes returns the response pkt-line as a slice
func (pu PackfileURI) Bytes() []byte {
	return pu.Append(nil)
}

// Parse populates the fields from a given pkt-line slice
func (pu *PackfileURI) Parse(line []byte) error {
	remaining, ok := bytes.CutSuffix(line, []byte("\n"))
	if !ok {
		return fmt.Errorf("invalid packfile-uri: %q", string(line))
	}
	checksum, uri, ok := bytes.Cut(remaining, []byte(" "))
	if !ok {
		return fmt.Errorf("invalid packfile-uri: %q", string(line))
	}
	pu.Checksum = string(checksum)
	pu.URI = string(uri)
	return nil
}

// packfile-uris = PKT-LINE("packfile-uris" LF) *packfile-uri
type PackfileURIs []PackfileURI

// IsZero returns true if the slice matches the zero value
func (pus PackfileURIs) IsZero() bool {
	return pus == nil
}

// Append the response pkt-lines to the given slice
func (pus PackfileURIs) Append(b []byte) []byte {
	b = pktline.AppendString(b, "packfile-uris\n")
	for _, pu := range pus {
		b = pu.Append(b)
	}
	return b
}

// Bytes returns the response pkt-line as a slice
func (pus PackfileURIs) Bytes() []byte {
	return pus.Append(nil)
}

// Parse populates the fields from a given pkt-line scanner
func (pus *PackfileURIs) Parse(scanner *pktline.Scanner) error {
	for {
		line, err := scanner.Scan()
		if err != nil {
			return err
		}
		var pu PackfileURI
		if err := pu.Parse(line); err != nil {
			return err
		}
		*pus = append(*pus, pu)
	}
}

// https://git-scm.com/docs/protocol-v2#_fetch
type FetchResponse struct {
	Acknowledgements Acknowledgements
	ShallowInfo      ShallowInfo
	WantedRefs       WantedRefs
	PackfileURIs     PackfileURIs
}

// Appends the response pkt-lines to the given slice
func (fr FetchResponse) Append(b []byte) []byte {
	if !fr.Acknowledgements.IsZero() {
		b = fr.Acknowledgements.Append(b)
		if fr.Acknowledgements.NAK {
			b = pktline.AppendFlushPkt(b)
			return b
		}
		b = pktline.AppendDelimPkt(b)
	}
	if !fr.ShallowInfo.IsZero() {
		b = fr.ShallowInfo.Append(b)
	}
	if !fr.WantedRefs.IsZero() {
		b = fr.WantedRefs.Append(b)
	}
	if !fr.PackfileURIs.IsZero() {
		b = fr.PackfileURIs.Append(b)
	}
	b = pktline.AppendString(b, "packfile\n")
	return b
}

// Bytes returns the response pkt-line as a slice
func (fr FetchResponse) Bytes() []byte {
	return fr.Append(nil)
}

// Parse populates the fields from a given pkt-line scanner
func (fr *FetchResponse) Parse(scanner *pktline.Scanner, packfile io.Writer, progress io.Writer) error {
	// TODO: This incorrectly permits a server to send sections out of order (or even more than once)
	for {
		line, err := scanner.Scan()
		if err != nil {
			if errors.Is(err, pktline.ErrDelimPkt) {
				continue
			}
			return err
		}
		switch {
		case bytes.Equal(line, []byte("acknowledgments\n")):
			log.Println("acknowledgments")
			err = fr.Acknowledgements.Parse(scanner)
		case bytes.Equal(line, []byte("shallow-info\n")):
			log.Println("shallow-info")
			err = fr.ShallowInfo.Parse(scanner)
		case bytes.Equal(line, []byte("wanted-refs\n")):
			log.Println("wanted-refs")
			err = fr.WantedRefs.Parse(scanner)
		case bytes.Equal(line, []byte("packfile-uris\n")):
			log.Println("packfile-uris")
			err = fr.PackfileURIs.Parse(scanner)
		case bytes.Equal(line, []byte("packfile\n")):
			for {
				line, err = scanner.Scan()
				if err != nil {
					if errors.Is(err, pktline.ErrFlushPkt) {
						return nil
					}
					return err
				}
				sideband, data := pktline.SideBand(line)
				switch sideband {
				case pktline.SideBandPackData:
					if packfile != nil {
						if _, err := packfile.Write(data); err != nil {
							return err
						}
					}
				case pktline.SideBandProgress:
					if progress != nil {
						if _, err := progress.Write(data); err != nil {
							return err
						}
					}
				case pktline.SideBandFatal:
					return fmt.Errorf("fatal: %s", string(data))
				default:
					return fmt.Errorf("invalid sideband: %q", string(line))
				}
			}
		default:
			err = fmt.Errorf("unsupported pkt-line: %q", string(line))
		}
		if err != nil {
			return err
		}
	}
}
