package yaml

func convertKeysToStrings(item interface{}) interface{} {
	switch typedDatas := item.(type) {

	case map[interface{}]interface{}:
		newMap := make(map[string]interface{})

		for key, value := range typedDatas {
			stringKey := key.(string)
			newMap[stringKey] = convertKeysToStrings(value)
		}
		return newMap

	case []interface{}:
		newArray := make([]interface{}, 0)
		for _, value := range typedDatas {
			newArray = append(newArray, convertKeysToStrings(value))
		}
		return newArray

	default:
		return item
	}
}
