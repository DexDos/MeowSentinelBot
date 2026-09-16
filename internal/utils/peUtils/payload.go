package peUtils

// peUtils refers to Payload Extraction Utils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func GetInfoFromCallbackData[T any](prefix string, cbd string) (T, error) {
	payload := strings.TrimPrefix(cbd, prefix)

	return GetInfoFromPayload[T](payload)
}
func GetInfoFromPayload[T any](payload string) (T, error) {
	var result T

	switch ptr := any(&result).(type) {
	case *string:
		*ptr = payload
	case *int64:
		i, err := strconv.ParseInt(payload, 10, 64)
		if err != nil {
			return result, err
		}
		*ptr = i
	default:
		panic(fmt.Sprintf("unsupported type: %T", result))
	}

	return result, nil
}

func ExtractSegment(input, delimiter string, targetPart int) (string, error) {
	if targetPart < 0 || targetPart > len(input) {
		return "", errors.New("target index is out of range")
	}

	parts := strings.Split(input, delimiter)
	if len(parts) < targetPart {
		return "", errors.New("target index is out of range")
	}
	return parts[targetPart], nil
}
