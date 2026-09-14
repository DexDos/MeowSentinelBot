package stackUtils

import "MeowSentinelBot/internal/model"

func SortStack(entries []model.Entry) [][]model.Entry {
	if len(entries) == 0 {
		return nil
	}

	typeIndex := make(map[string]int)
	result := make([][]model.Entry, 0)

	for _, entry := range entries {
		idx, exist := typeIndex[entry.Type]
		if !exist {
			idx = len(result)
			typeIndex[entry.Type] = idx
			result = append(result, []model.Entry{entry})
			continue
		}
		result[idx] = append(result[idx], entry)
	}
	return result
}
