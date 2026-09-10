package utils

import (
	"encoding/json"
	"fmt"
	"go/types"
	"io"
	"log"
	"os"
	"strings"
)

func ParseJSONFromFile(filepath string, v any) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			return fmt.Errorf("fatal while closing the config file: %w", err)
		}
	}(f)
	err = json.NewDecoder(f).Decode(v)

	return err
}

func ParseJSONFromBytes(bytes []byte, v any) error {
	if err := json.Unmarshal(bytes, v); err != nil {
		return fmt.Errorf("unmarshal json bytes error: %w", err)
	}
	return nil
}

func ParseJSONFromString(str string, v any) error {
	if err := json.NewDecoder(strings.NewReader(str)).Decode(v); err != nil {
		return fmt.Errorf("decode json string error: %w", err)
	}
	return nil
}
