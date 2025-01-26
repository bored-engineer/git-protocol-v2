package protocolv2

import (
	"bytes"
	"errors"
	"fmt"

	pktline "github.com/bored-engineer/git-pkt-line"
)

// capability-advertisement = protocol-version capability-list flush-pkt
// protocol-version = PKT-LINE("version 2" LF)
// capability-list = *capability
type CapabilityAdvertisement struct {
	Capabilities Capabilities
}

// Bytes returns the advertisement pkt-lines to the given slice
func (ca CapabilityAdvertisement) Append(b []byte) []byte {
	b = pktline.AppendString(b, "version 2\n")
	b = ca.Capabilities.Append(b)
	b = pktline.AppendFlushPkt(b)
	return b
}

// Bytes returns the advertisement pkt-lines as a slice
func (ca CapabilityAdvertisement) Bytes() []byte {
	return ca.Append(nil)
}

// Parse populates the fields from a given scanner
func (ca *CapabilityAdvertisement) Parse(scanner *pktline.Scanner) error {
	version, err := scanner.Scan()
	if err != nil {
		return err
	}
	if !bytes.Equal(version, []byte("version 2\n")) {
		return fmt.Errorf("invalid protocol-version: %q", string(version))
	}
	if err := ca.Capabilities.Parse(scanner); err != nil && !errors.Is(err, pktline.ErrFlushPkt) {
		return err
	}
	return nil
}
