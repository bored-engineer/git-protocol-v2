package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	pktline "github.com/bored-engineer/git-pkt-line"
	git "github.com/bored-engineer/git-protocol-v2"
	"github.com/spf13/pflag"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	want := pflag.StringSlice("want", nil, "Indicates to the server an object which the client wants to retrieve. Wants can be anything and are not limited to advertised objects.")
	have := pflag.StringSlice("have", nil, "Indicates to the server an object which the client has locally. This allows the server to make a packfile which only contains the objects that the client needs. Multiple 'have' lines can be supplied.")
	thinPack := pflag.Bool("thin-pack", false, "Request that a thin pack be sent, which is a pack with deltas which reference base objects not contained within the pack (but are known to exist at the receiving end). This can reduce the network traffic significantly, but it requires the receiving end to know how to \"thicken\" these packs by adding the missing bases to the pack.")
	noProgress := pflag.Bool("no-progress", false, "Request that progress information that would normally be sent on side-band channel 2, during the packfile transfer, should not be sent. However, the side-band channel 3 is still used for error responses.")
	includeTag := pflag.Bool("include-tag", false, "Request that annotated tags should be sent if the objects they point to are being sent.")
	ofsDelta := pflag.Bool("ofs-delta", false, "Indicate that the client understands PACKv2 with delta referring to its base by position in pack rather than by an oid. That is, they can read OBJ_OFS_DELTA (aka type 6) in a packfile.")
	shallows := pflag.StringSlice("shallow", nil, "A client must notify the server of all commits for which it only has shallow copies (meaning that it doesn't have the parents of a commit) by supplying a 'shallow <oid>' line for each such object so that the server is aware of the limitations of the client's history.")
	deepen := pflag.String("deepen", "", "Requests that the fetch/clone should be shallow having a commit depth of <depth> relative to the remote side.")
	deepenRelative := pflag.Bool("deepen-relative", false, "Requests that the semantics of the 'deepen' command be changed to indicate that the depth requested is relative to the client's current shallow boundary, instead of relative to the requested commits.")
	deepenSince := pflag.String("deepen-since", "", "Requests that the shallow clone/fetch should be cut at a specific time, instead of depth. Internally it's equivalent to doing 'git rev-list --max-age=<timestamp>'. Cannot be used with 'deepen'.")
	deepenNot := pflag.String("deepen-not", "", "Requests that the shallow clone/fetch should be cut at a specific revision specified by '<rev>', instead of a depth. Internally it's equivalent of doing 'git rev-list --not <rev>'. Cannot be used with 'deepen', but can be used with 'deepen-since'.")
	filter := pflag.String("filter", "", "Request that various objects from the packfile be omitted using one of several filtering techniques. These are intended for use with partial clone and partial fetch operations. See `rev-list` for possible 'filter-spec' values. When communicating with other processes, senders SHOULD translate scaled integers (e.g. '1k') into a fully-expanded form (e.g. '1024') to aid interoperability with older receivers that may not understand newly-invented scaling suffixes. However, receivers SHOULD accept the following suffixes: 'k', 'm', and 'g' for 1024, 1048576, and 1073741824, respectively.")
	wantRefs := pflag.StringSlice("want-ref", nil, "Indicates to the server that the client wants to retrieve a particular ref, where <ref> is the full name of a ref on the server.")
	packfileURIs := pflag.StringSlice("packfile-uris", nil, "Indicates to the server that the client is willing to receive URIs of any of the given protocols in place of objects in the sent packfile. Before performing the connectivity check, the client should download from all given URIs. Currently, the protocols supported are 'http' and 'https'.")
	stdin := pflag.Bool("stdin", false, "Read the 'want' lines from stdin instead of '--want'.")
	capabilities := pflag.StringSlice("capability", nil, "Advertise a client capability in the command-request.")
	userAgent := pflag.String("user-agent", "git/1.0", "Set the User-Agent header in the HTTP request.")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <url>\n", filepath.Base(os.Args[0]))
		pflag.PrintDefaults()
	}
	pflag.Parse()
	if pflag.NArg() != 1 {
		pflag.Usage()
		os.Exit(1)
	}
	url := pflag.Arg(0) + "/git-upload-pack"

	// The --stdin flag allows us to add 'wants' directly piped from the output of 'ls-refs'
	if *stdin {
		scanner := bufio.NewScanner(os.Stdin)
		uniq := make(map[string]struct{})
		for scanner.Scan() {
			oid, _, _ := strings.Cut(scanner.Text(), " ")
			uniq[oid] = struct{}{}
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("bufio.Scanner.Scan stdin failed: %v", err)
		}
		for oid := range uniq {
			*want = append(*want, oid)
		}
	}
	if len(*want) == 0 && len(*wantRefs) == 0 {
		fmt.Fprintln(os.Stderr, "At least one '--want' or '--want-ref' is required")
		os.Exit(1)
	}

	req := git.CommandRequest{
		Command: "fetch",
		Arguments: git.CommandArguments{
			{
				// We aren't doing true negotiation here, so tell the server to wait for us to finish sending our have/want lines before responding.
				Key: git.ArgumentWaitForDone,
			},
		},
	}

	for _, cap := range *capabilities {
		key, value, _ := strings.Cut(cap, "=")
		req.Capabilities = append(req.Capabilities, git.Capability{
			Key:   key,
			Value: value,
		})
	}

	if *thinPack {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key: git.ArgumentThinPack,
		})
	}
	if *noProgress {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key: git.ArgumentNoProgress,
		})
	}
	if *includeTag {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key: git.ArgumentIncludeTag,
		})
	}
	if *ofsDelta {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key: git.ArgumentOFSDelta,
		})
	}
	for _, shallow := range *shallows {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentShallow,
			Value: shallow,
		})
	}
	if *deepen != "" {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentDeepen,
			Value: *deepen,
		})
	}
	if *deepenRelative {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key: git.ArgumentDeepenRelative,
		})
	}
	if *deepenSince != "" {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentDeepenSince,
			Value: *deepenSince,
		})
	}
	if *deepenNot != "" {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentDeepenNot,
			Value: *deepenNot,
		})
	}
	if *filter != "" {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentFilter,
			Value: *filter,
		})
	}
	for _, ref := range *wantRefs {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentWantRef,
			Value: ref,
		})
	}
	if len(*packfileURIs) > 0 {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentPackfileURIs,
			Value: strings.Join(*packfileURIs, ","),
		})
	}

	// "negotiation" phase
	for _, oid := range *have {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentHave,
			Value: oid,
		})
	}
	for _, oid := range *want {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentWant,
			Value: oid,
		})
	}
	req.Arguments = append(req.Arguments, git.CommandArgument{
		Key: git.ArgumentDone,
	})

	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(req.Bytes()))
	if err != nil {
		log.Fatalf("http.NewRequest failed: %v", err)
	}
	reqHTTP.Header.Set("Git-Protocol", "version=2")
	reqHTTP.Header.Set("User-Agent", *userAgent)

	respHTTP, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		log.Fatalf("http.DefaultClient.Do failed: %v", err)
	}
	defer func() {
		if err := respHTTP.Body.Close(); err != nil {
			log.Fatalf("(*http.Response).Body.Close failed: %v", err)
		}
	}()

	if respHTTP.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respHTTP.Body)
		log.Fatalf("unexpected status code (%d): %s", respHTTP.StatusCode, string(body))
	}

	scanner := pktline.NewScanner(respHTTP.Body)

	var resp git.FetchResponse
	if err := resp.Parse(scanner, os.Stdout, os.Stderr); err != nil {
		log.Fatalf("failed to parse fetch response: %v", err)
	}

	if resp.Acknowledgements.Ready {
		fmt.Fprintf(os.Stderr, "Ready\n")
	}
	if resp.Acknowledgements.NAK {
		fmt.Fprintf(os.Stderr, "NAK\n")
	}
	for _, ack := range resp.Acknowledgements.ACKs {
		fmt.Fprintf(os.Stderr, "ACK %s\n", ack)
	}
	for _, shallow := range resp.ShallowInfo.Shallow {
		fmt.Fprintf(os.Stderr, "shallow %s\n", shallow.ObjectID)
	}
	for _, unshallow := range resp.ShallowInfo.Unshallow {
		fmt.Fprintf(os.Stderr, "unshallow %s\n", unshallow.ObjectID)
	}
	for _, wantedRef := range resp.WantedRefs {
		fmt.Fprintf(os.Stderr, "wanted-ref %s %s\n", wantedRef.ObjectID, wantedRef.Name)
	}
	for _, packfileURI := range resp.PackfileURIs {
		fmt.Fprintf(os.Stderr, "packfile-uri %s\n", packfileURI)
	}

}
