package operationplan

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func Load(path string) (Set, error) {
	file, err := os.Open(path)
	if err != nil {
		return Set{}, err
	}
	defer file.Close()
	return Decode(file)
}

func Decode(reader io.Reader) (Set, error) {
	if reader == nil {
		return Set{}, fmt.Errorf("operationplan: reader is required")
	}
	var set Set
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&set); err != nil {
		return Set{}, fmt.Errorf("operationplan: decode: %w", err)
	}
	set = Normalize(set)
	if err := Validate(set); err != nil {
		return Set{}, err
	}
	return set, nil
}

func Encode(writer io.Writer, set Set) error {
	if writer == nil {
		return fmt.Errorf("operationplan: writer is required")
	}
	data, err := CanonicalJSON(set)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}
