package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type graph struct {
	root     *vertex
	vertices map[string]bool
}

type vertex struct {
	name  string
	edges map[string]*vertex
}

func newVertex(name string) *vertex {
	return &vertex{name: name, edges: map[string]*vertex{}}
}

// newGraph builds a graph from a Reader that has data formatted in the format:
// A B
// Where A has a directed edge to B.
func newGraph(in io.Reader) (*graph, error) {
	vertexMap := map[string]*vertex{}
	var root string
	r := bufio.NewScanner(in)
	for r.Scan() {
		parts := strings.Fields(r.Text())
		if r.Text() == "" {
			// Ignore empty lines.
			continue
		}
		if len(parts) != 2 {
			return nil, fmt.Errorf("couldn't decipher %s", r.Text())
		}

		fromName := parts[0]
		if root == "" {
			root = fromName
		}

		toName := parts[1]
		fromVertex, ok := vertexMap[fromName]
		if !ok {
			// fromVertex is nil - we couldn't find it. This could happen if
			// the list is unordered: B->C, A->B.
			fromVertex = newVertex(fromName)
			vertexMap[fromVertex.name] = fromVertex
		}

		toVertex, ok := vertexMap[toName]
		if !ok {
			toVertex = newVertex(toName)
			vertexMap[toVertex.name] = toVertex
		}

		fromVertex.edges[toVertex.name] = toVertex
	}
	if r.Err() != nil {
		return nil, r.Err()
	}

	vertices := map[string]bool{}
	for k := range vertexMap {
		vertices[k] = true
	}

	return &graph{
		root:     vertexMap[root],
		vertices: vertices,
	}, nil
}

// print prints the graph.
func (g *graph) print(out io.Writer) error {
	return g.printDFS(out, map[string]bool{}, g.root)
}

// printDFS traverses the graph depth first, printing each edge.
func (g *graph) printDFS(out io.Writer, visited map[string]bool, cursor *vertex) error {
	if visited[cursor.name] {
		return nil // Stop if we've already visited this vertex.
	}
	visited[cursor.name] = true
	for _, edge := range cursor.edges {
		if _, err := fmt.Fprintf(out, "\t%q -> %q\n", cursor.name, edge.name); err != nil {
			return err
		}
		if err := g.printDFS(out, visited, edge); err != nil {
			return err
		}
	}

	return nil
}

type breadcrumb struct {
	*vertex             // Current vertex.
	from    *breadcrumb // The vertex we traveled from to get here.
}

/*
b1 -- b2 -- b3
b1.5 /
*/
func (b *breadcrumb) hasCycle() bool {
	cursor := &breadcrumb{vertex: b.vertex, from: b.from}
	seen := map[string]bool{cursor.name: true}
	for {
		cursor = cursor.from
		if cursor == nil {
			return false
		}
		if seen[cursor.name] {
			return true
		}
	}
}

// to prints the paths from g.root to the needle.
func (g *graph) to(needle string) (*graph, error) {
	if _, ok := g.vertices[needle]; !ok {
		return nil, fmt.Errorf("%q does not exist in dependency graph", needle)
	}

	newRoot := newVertex(g.root.name)
	//visited := map[string]bool{newRoot.name: true}
	vertexMap := map[string]*vertex{newRoot.name: newRoot}
	q := []*breadcrumb{{vertex: g.root}}

	// n = nodes in original graph
	// O(n)
	for len(q) > 0 {
		cursor := q[0]
		q = q[1:]

		// O(n)
		// TODO: can we replace this O(n) traversal with a O(1) map lookup
		if cursor.hasCycle() {
			continue
		}

		if cursor.name == needle {
			// Last element should always be the connecting piece.
			var path []*vertex

			// O(n)
			for crumbCursor := cursor; crumbCursor != nil; crumbCursor = crumbCursor.from { // Going up.
				path = append(path, crumbCursor.vertex)
			}

			// O(n)
			for i := len(path) - 1; i > 0; i-- { // Going down.
				var ok bool
				var from *vertex

				// TODO: maybe from already exists?
				from, ok = vertexMap[path[i].name]
				if !ok {
					from = newVertex(path[i].name)
					vertexMap[from.name] = from
				}
				var to *vertex
				to, ok = vertexMap[path[i-1].name]
				if !ok {
					to = newVertex(path[i-1].name)
					vertexMap[to.name] = to
				}

				if _, ok := from.edges[to.name]; !ok {
					from.edges[to.name] = to
				}
			}
		}

		for _, edge := range cursor.edges {
			q = append(q, &breadcrumb{
				vertex: edge,
				from:   cursor,
			})
		}
	}

	vertices := map[string]bool{}
	for k := range vertexMap {
		vertices[k] = true
	}

	return &graph{root: newRoot, vertices: vertices}, nil
}
