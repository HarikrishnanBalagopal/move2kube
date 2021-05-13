/*
Copyright IBM Corporation 2020

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/konveyor/move2kube/internal/common"
	"github.com/spf13/cast"
)

type mapT = map[string]interface{}

var (
	arrayIndexRegex    = regexp.MustCompile(`^\[(\d+)\]$`)
	complexSubKeyRegex = regexp.MustCompile(`^\[(\w+:)?(\w+)(=.+)?\]$`)
)

type RT struct {
	Key     []string
	Value   interface{}
	Matches map[string]string
}

func isNormal(k string) bool {
	return !strings.Contains(k, "[") || arrayIndexRegex.MatchString(k)
}

// GetAll returns all the keys that matched and all corresponding values
func GetAll(key string, resource interface{}) ([]RT, error) {
	results := []RT{}
	subKeys := GetSubKeys(key)
	currentResult := RT{}
	err := GetRecurse(subKeys, 0, resource, currentResult, &results)
	return results, err
}

// GetRecurse recurses on the value and finds all matches for the key
func GetRecurse(subKeys []string, subKeyIdx int, value interface{}, currentResult RT, results *[]RT) error {
	if subKeyIdx >= len(subKeys) {
		kc := make([]string, len(currentResult.Key))
		copy(kc, currentResult.Key)
		currentResult.Key = kc
		currentResult.Value = value
		*results = append(*results, currentResult)
		return nil
	}
	subKey := subKeys[subKeyIdx]
	if isNormal(subKey) {
		valueMap, ok := value.(mapT)
		if ok {
			value, ok = valueMap[subKey]
			if ok {
				currentResult.Key = append(currentResult.Key, subKey)
				return GetRecurse(subKeys, subKeyIdx+1, value, currentResult, results)
			}
			return fmt.Errorf("failed to find the subkey %s in the map %+v", subKey, valueMap)
		}
		valueArr, ok := value.([]interface{})
		if ok {
			idx, ok := getIndex(subKey)
			if !ok {
				return fmt.Errorf("failed to interpret the subkey %s as an index to the slice %+v", subKey, valueArr)
			}
			if idx >= len(valueArr) {
				return fmt.Errorf("the index %d is out of range for the slice %+v", idx, valueArr)
			}
			value = valueArr[idx]
			currentResult.Key = append(currentResult.Key, subKey)
			return GetRecurse(subKeys, subKeyIdx+1, value, currentResult, results)
		}
		return fmt.Errorf("the value is not a map or slice. Actual value %+v is of type %T", value, value)
	}
	// subkey like [containerName:name=nginx]
	if !complexSubKeyRegex.MatchString(subKey) {
		return fmt.Errorf("the subkey %s is invalid", subKey)
	}
	subMatches := complexSubKeyRegex.FindAllStringSubmatch(subKey, -1)
	if len(subMatches) != 1 {
		return fmt.Errorf("expected there to be 1 match. Actual no. of matches %d matches: %+v", len(subMatches), subMatches)
	}
	if len(subMatches[0]) != 4 {
		return fmt.Errorf("expected there to be 4 submatches. Actual no. of submatches %d submatches: %+v", len(subMatches[0]), subMatches[0])
	}
	matchName, matchKey, matchValue := subMatches[0][1], subMatches[0][2], subMatches[0][3]
	if matchName == "" {
		matchName = matchKey
	} else {
		matchName = strings.TrimSuffix(matchName, ":")
	}
	if matchValue != "" {
		matchValue = strings.TrimPrefix(matchValue, "=")
	}
	valueArr, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("expected a slice of objects. actual value is %+v of type %T", value, value)
	}
	if len(valueArr) == 0 {
		return nil
	}
	for arrIdx, valueMapI := range valueArr {
		valueMap, ok := valueMapI.(mapT)
		if !ok {
			return fmt.Errorf("expected all the elements of the slice to be object. actual value is %+v of %T", valueMapI, valueMapI)
		}
		actualMatchValueI, ok := valueMap[matchKey]
		if !ok {
			continue
		}
		actualMatchValue, ok := actualMatchValueI.(string)
		if !ok {
			return fmt.Errorf("expected the value to be a string. Actual value is %+v of type %T", actualMatchValueI, actualMatchValueI)
		}
		if matchValue != "" && matchValue != actualMatchValue {
			continue
		}
		if currentResult.Matches == nil {
			currentResult.Matches = map[string]string{}
		}
		orig := currentResult.Matches
		copy := map[string]string{}
		for k, v := range orig {
			copy[k] = v
		}
		copy[matchName] = actualMatchValue
		currentResult.Matches = copy
		origKey := currentResult.Key
		currentResult.Key = append(origKey, "["+cast.ToString(arrIdx)+"]")
		if err := GetRecurse(subKeys, subKeyIdx+1, valueArr[arrIdx], currentResult, results); err != nil {
			return err
		}
		currentResult.Matches = orig
		currentResult.Key = origKey
	}
	return nil
}

// Get returns the value at the key in the config
func Get(key string, config interface{}) (value interface{}, ok bool) {
	subKeys := GetSubKeys(key)
	value = config
	for _, subKey := range subKeys {
		valueMap, ok := value.(mapT)
		if ok {
			value, ok = valueMap[subKey]
			if ok {
				continue
			}
			return value, false
		}
		valueArr, ok := value.([]interface{})
		if ok {
			idx, ok := getIndex(subKey)
			if ok && idx < len(valueArr) {
				value = valueArr[idx]
				continue
			}
		}
		return value, false
	}
	return value, true
}

// Set updates the value at the key in the config with the new value
func Set(key string, newValue, config interface{}) error {
	if key == "" {
		return fmt.Errorf("the key is an empty string")
	}
	subKeys := GetSubKeys(key)
	if len(subKeys) == 0 {
		return fmt.Errorf("no sub keys found for the key %s", key)
	}
	value := config
	for _, subKey := range subKeys[:len(subKeys)-1] {
		valueMap, ok := value.(mapT)
		if ok {
			value, ok = valueMap[subKey]
			if ok {
				continue
			}
			return fmt.Errorf("the sub key %s is not present in the map %+v", subKey, valueMap)
		}
		valueArr, ok := value.([]interface{})
		if ok {
			idx, ok := getIndex(subKey)
			if ok && idx < len(valueArr) {
				value = valueArr[idx]
				continue
			}
			return fmt.Errorf("the sub key %s is not a valid index into the array %+v", subKey, valueArr)
		}
		return fmt.Errorf("the sub key %s cannot be matched because we reached a scalar value %+v", subKey, value)
	}
	subKey := subKeys[len(subKeys)-1]
	if valueMap, ok := value.(mapT); ok {
		if _, ok := valueMap[subKey]; ok {
			valueMap[subKey] = newValue
			return nil
		}
		return fmt.Errorf("the sub key %s is not present in the map %+v", subKey, valueMap)
	}
	if valueArr, ok := value.([]interface{}); ok {
		idx, ok := getIndex(subKey)
		if ok && idx < len(valueArr) {
			valueArr[idx] = newValue
			return nil
		}
		return fmt.Errorf("the sub key %s is not a valid index into the array %+v", subKey, valueArr)
	}
	return fmt.Errorf("expected a map or array type. Actual value is %+v of type %T", value, value)
}

// SetCreatingNew updates the value at the key in the config with the new value
func SetCreatingNew(key string, newValue interface{}, config mapT) error {
	if key == "" {
		return fmt.Errorf("the key is an empty string")
	}
	subKeys := GetSubKeys(key)
	if len(subKeys) == 0 {
		return fmt.Errorf("no sub keys found for the key %s", key)
	}
	lastIdx := len(subKeys) - 1
	var value interface{}
	var ok bool
	for _, subKey := range subKeys[:lastIdx] {
		value, ok = config[subKey]
		if !ok {
			// sub key doesn't exist
			newMap := mapT{}
			config[subKey] = newMap
			config = newMap
			continue
		}
		valueMap, ok := value.(mapT)
		if ok {
			config = valueMap
			continue
		}
		// sub key exists but corresponding value is not a map
		newMap := mapT{}
		config[subKey] = newMap
		config = newMap
	}
	lastSubKey := subKeys[lastIdx]
	config[lastSubKey] = newValue
	return nil
}

// GetSubKeys returns the parts of a key.
// Example aaa.bbb."ccc ddd".eee.fff -> {"aaa", "bbb", "ccc ddd", "eee", "fff"}
func GetSubKeys(key string) []string {
	unStrippedSubKeys := common.SplitOnDotExpectInsideQuotes(key) // assuming delimiter is dot
	subKeys := []string{}
	for _, unStrippedSubKey := range unStrippedSubKeys {
		subKeys = append(subKeys, common.StripQuotes(unStrippedSubKey))
	}
	return subKeys
}

func getIndex(key string) (int, bool) {
	matches := arrayIndexRegex.FindSubmatch([]byte(key))
	if matches == nil {
		return 0, false
	}
	idx, err := cast.ToIntE(string(matches[1]))
	if err != nil || idx < 0 {
		return 0, false
	}
	return idx, true
}
