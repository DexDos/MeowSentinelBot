package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bytedance/gopkg/util/logger"
)

func ParseJSONFromFile(filepath string, v any) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			logger.Errorf("error closing file: %v", err)
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
