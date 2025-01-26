package main

import (
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

	symrefs := pflag.Bool("symrefs", false, "In addition to the object pointed by it, show the underlying ref pointed by it when showing a symbolic ref.")
	peel := pflag.Bool("peel", false, "Show peeled tags.")
	unborn := pflag.Bool("unborn", false, "request unborn refs")
	refPrefixes := pflag.StringSlice("ref-prefix", nil, "When specified, only references having a prefix matching one of the provided prefixes are displayed. Multiple instances may be given, in which case references matching any prefix will be shown. Note that this is purely for optimization; a server MAY show refs not matching the prefix if it chooses, and clients should filter the result themselves.")
	capabilities := pflag.StringSlice("capability", nil, "Advertise a client capability in the command-request.")
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

	req := git.CommandRequest{
		Command: "ls-refs",
	}
	for _, cap := range *capabilities {
		key, value, _ := strings.Cut(cap, "=")
		req.Capabilities = append(req.Capabilities, git.Capability{
			Key:   key,
			Value: value,
		})
	}

	if *symrefs {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key: git.ArgumentSymRefs,
		})
	}
	if *peel {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key: git.ArgumentPeel,
		})
	}
	if *unborn {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key: git.ArgumentUnborn,
		})
	}
	for _, prefix := range *refPrefixes {
		req.Arguments = append(req.Arguments, git.CommandArgument{
			Key:   git.ArgumentRefPrefix,
			Value: prefix,
		})
	}

	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(req.Bytes()))
	if err != nil {
		log.Fatalf("http.NewRequestWithContext failed: %v", err)
	}
	reqHTTP.Header.Set("Git-Protocol", "version=2")

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

	var resp git.ListReferencesResponse
	if err := resp.Parse(scanner); err != nil {
		log.Fatalf("failed to parse ls-refs response: %v", err)
	}
	for _, ref := range resp.References {
		fmt.Println(ref.String())
	}

}
