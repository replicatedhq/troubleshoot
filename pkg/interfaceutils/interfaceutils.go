package interfaceutils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func GetAtPath(input interface{}, path string) (interface{}, error) {
	parts := strings.SplitN(path, ".", 2)
	key := parts[0]
	if isArrayIndex(key) {
		i, err := getArrayIndexValue(key)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get index value of %s", key)
		}
		obj, ok := input.([]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf("input is not an array: %+v", input))
		}
		input = obj[i]
	} else {
		switch t := input.(type) {
		case map[interface{}]interface{}:
			input = input.(map[interface{}]interface{})[key]
		case map[string]interface{}:
			input = input.(map[string]interface{})[key]
		default:
			return nil, errors.New(fmt.Sprintf("input is not a map, but rather a %v: %+v", t, input))
		}
	}

	if len(parts) > 1 {
		return GetAtPath(input, parts[1])
	}

	return input, nil
}

func isArrayIndex(key string) bool {
	return strings.HasPrefix(key, "[") && strings.HasSuffix(key, "]")
}

func getArrayIndexValue(key string) (int, error) {
	key = strings.TrimPrefix(key, "[")
	key = strings.TrimSuffix(key, "]")
	i, err := strconv.Atoi(key)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to parse index %s", key)
	}
	return i, nil
}
