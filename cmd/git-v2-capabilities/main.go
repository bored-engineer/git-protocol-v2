package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"

	pktline "github.com/bored-engineer/git-pkt-line"
	git "github.com/bored-engineer/git-protocol-v2"
	"github.com/spf13/pflag"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	service := pflag.String("service", "git-upload-pack", "service parameter in the query string")
	smart := pflag.Bool("smart", true, "expect smart HTTP protocol response")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <url>\n", filepath.Base(os.Args[0]))
		pflag.PrintDefaults()
	}
	pflag.Parse()
	if pflag.NArg() != 1 {
		pflag.Usage()
		os.Exit(1)
	}
	url := pflag.Arg(0) + "/info/refs?service=" + *service

	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Fatalf("http.NewRequest failed: %v", err)
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
	if *smart {
		if smartHTTP, err := scanner.Scan(); err != nil {
			log.Fatalf("scanner.Scan failed: %v", err)
		} else if !bytes.Equal(smartHTTP, []byte("# service="+*service+"\n")) {
			log.Fatalf("unexpected smart-http response: %q", string(smartHTTP))
		}
		if line, err := scanner.Scan(); !errors.Is(err, pktline.ErrFlushPkt) {
			log.Fatalf("expected flush-pkt (%v), got: %q", err, string(line))
		}
	}

	var resp git.CapabilityAdvertisement
	if err := resp.Parse(scanner); err != nil {
		log.Fatalf("failed to parse capability-advertisement: %v", err)
	}
	for _, cap := range resp.Capabilities {
		fmt.Println(cap.String())
	}

}
