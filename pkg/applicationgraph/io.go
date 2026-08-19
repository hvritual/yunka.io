package applicationgraph

import (
	"encoding/json"
	"io"
	"os"
)

func Load(path string) (Graph, error) {
	file, err := os.Open(path)
	if err != nil {
		return Graph{}, err
	}
	defer file.Close()
	return Decode(file)
}

func Decode(reader io.Reader) (Graph, error) {
	var graph Graph
	if err := json.NewDecoder(reader).Decode(&graph); err != nil {
		return Graph{}, err
	}
	if err := graph.Validate(); err != nil {
		return Graph{}, err
	}
	return graph, nil
}

func Encode(writer io.Writer, graph Graph) error {
	if err := graph.Validate(); err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(graph)
}
