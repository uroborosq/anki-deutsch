// Command wikt-import trims a raw kaikki.org German Wiktionary (dewiktionary)
// JSONL dump into the compact form the offline lexicon Client loads: one
// JSON-encoded lexicon.Word per German lemma, with the heavy inflection tables,
// examples and sounds dropped. Run it once after downloading the dump.
//
// Usage:
//
//	wikt-import -in kaikki.org-dictionary-Deutsch.jsonl -out de-compact.jsonl
//
// The input may be plain .jsonl or .jsonl.gz / .jsonl.bz2 (detected by suffix).
package main

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"anki/internal/lexicon/wiktextract"
)

func main() {
	in := flag.String("in", "", "raw kaikki Deutsch JSONL dump (.jsonl, .gz or .bz2)")
	out := flag.String("out", "", "compact output JSONL path")
	flag.Parse()
	if *in == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*in, *out); err != nil {
		fmt.Fprintln(os.Stderr, "wikt-import:", err)
		os.Exit(1)
	}
}

func run(inPath, outPath string) error {
	f, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = bufio.NewReaderSize(f, 1<<20)
	switch {
	case strings.HasSuffix(inPath, ".gz"):
		gz, err := gzip.NewReader(r)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	case strings.HasSuffix(inPath, ".bz2"):
		r = bzip2.NewReader(r)
	}

	of, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer of.Close()
	bw := bufio.NewWriterSize(of, 1<<20)
	enc := json.NewEncoder(bw)

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24) // some verb entries are hundreds of KB

	seen := map[string]bool{}
	var read, kept int
	for sc.Scan() {
		read++
		w, ok := wiktextract.ParseEntry(sc.Bytes())
		if !ok || seen[w.Lemma] {
			continue
		}
		seen[w.Lemma] = true
		if err := enc.Encode(w); err != nil {
			return err
		}
		kept++
		if read%200000 == 0 {
			fmt.Fprintf(os.Stderr, "\r%d read, %d lemmas kept", read, kept)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\rdone: %d read, %d lemmas written to %s\n", read, kept, outPath)
	return nil
}
