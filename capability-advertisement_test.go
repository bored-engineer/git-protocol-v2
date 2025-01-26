package protocolv2

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	pktline "github.com/bored-engineer/git-pkt-line"
)

var payloadCapabilityAdvertisement = `000eversion 2
0022agent=git/github-8e2ff7c5586f
0013ls-refs=unborn
0027fetch=shallow wait-for-done filter
0012server-option
0017object-format=sha1
0000`

func TestCapabilityAdvertisement(t *testing.T) {
	scanner := pktline.NewScanner(strings.NewReader(payloadCapabilityAdvertisement))
	var ca CapabilityAdvertisement
	if err := ca.Parse(scanner); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ca.Capabilities, Capabilities{
		{"agent", "git/github-8e2ff7c5586f"},
		{"ls-refs", "unborn"},
		{"fetch", "shallow wait-for-done filter"},
		{"server-option", ""},
		{"object-format", "sha1"},
	}) {
		t.Fatalf("expected capabilities to match")
	}
	if !bytes.Equal(ca.Bytes(), []byte(payloadCapabilityAdvertisement)) {
		t.Fatalf("expected payload to match")
	}
}
