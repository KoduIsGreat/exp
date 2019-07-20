// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Modgraphviz converts “go mod graph” output into Graphviz's DOT language,
// for use with Graphviz visualization and analysis tools like dot, dotty, and sccmap.
//
// Usage:
//
//	go mod graph | modgraphviz > graph.dot
//	go mod graph | modgraphviz | dot -Tpng -o graph.png
//
// Modgraphviz takes no options or arguments; it reads a graph in the format
// generated by “go mod graph” on standard input and writes DOT language
// on standard output.
//
// For each module, the node representing the greatest version (i.e., the
// version chosen by Go's minimal version selection algorithm) is colored green.
// Other nodes, which aren't in the final build list, are colored grey.
//
// See http://www.graphviz.org/doc/info/lang.html for details of the DOT language
// and http://www.graphviz.org/about/ for Graphviz itself.
//
// See also golang.org/x/tools/cmd/digraph for general queries and analysis
// of “go mod graph” output.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: go mod graph | modgraphviz | dot -Tpng -o graph.png

For each module, the node representing the greatest version (i.e., the
version chosen by Go's minimal version selection algorithm) is colored green.
Other nodes, which aren't in the final build list, are colored grey.
`)
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("modgraphviz: ")

	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 0 {
		usage()
	}

	if err := modgraphviz(os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func modgraphviz(in io.Reader, out io.Writer) error {
	graph, err := convert(in)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "digraph gomodgraph {\n")
	fmt.Fprintf(out, "\tnode [ shape=rectangle fontsize=12 ]\n")
	out.Write(graph.edgesAsDOT())
	for _, n := range graph.mvsPicked {
		fmt.Fprintf(out, "\t%q [style = filled, fillcolor = green]\n", n)
	}
	for _, n := range graph.mvsUnpicked {
		fmt.Fprintf(out, "\t%q [style = filled, fillcolor = gray]\n", n)
	}
	fmt.Fprintf(out, "}\n")

	return nil
}

type edge struct{ from, to string }
type graph struct {
	edges       []edge
	mvsPicked   []string
	mvsUnpicked []string
}

// convert reads “go mod graph” output from r and returns a graph, recording
// MVS picked and unpicked nodes along the way.
func convert(r io.Reader) (*graph, error) {
	scanner := bufio.NewScanner(r)
	var g graph
	seen := map[string]bool{}
	mvsPicked := map[string]string{} // module name -> module version

	for scanner.Scan() {
		l := scanner.Text()
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) != 2 {
			return nil, fmt.Errorf("expected 2 words in line, but got %d: %s", len(parts), l)
		}
		from := strings.Replace(parts[0], "@", "\n@", 1)
		to := strings.Replace(parts[1], "@", "\n@", 1)
		g.edges = append(g.edges, edge{from: from, to: to})

		for _, node := range []string{from, to} {
			if _, ok := seen[node]; ok {
				// Skip over nodes we've already seen.
				continue
			}
			seen[node] = true

			var m, v string
			if i := strings.IndexByte(node, '@'); i >= 0 {
				m, v = node[:i], node[i+1:]
			} else {
				// Root node doesn't have a version.
				continue
			}

			if maxV, ok := mvsPicked[m]; ok {
				if semver.Compare(maxV, v) < 0 {
					// This version is higher - replace it and consign the old
					// max to the unpicked list.
					g.mvsUnpicked = append(g.mvsUnpicked, m+"@"+maxV)
					mvsPicked[m] = v
				} else {
					// Other version is higher - stick this version in the
					// unpicked list.
					g.mvsUnpicked = append(g.mvsUnpicked, node)
				}
			} else {
				mvsPicked[m] = v
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for m, v := range mvsPicked {
		g.mvsPicked = append(g.mvsPicked, m+"@"+v)
	}

	// Make this function deterministic.
	sort.Strings(g.mvsPicked)
	return &g, nil
}

// edgesAsDOT returns the edges in DOT notation.
func (g *graph) edgesAsDOT() []byte {
	var buf bytes.Buffer
	for _, e := range g.edges {
		fmt.Fprintf(&buf, "\t%q -> %q\n", e.from, e.to)
	}
	return buf.Bytes()
}
