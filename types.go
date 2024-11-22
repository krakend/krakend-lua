package lua

import (
	"errors"
	"sort"
	"strconv"

	glua "github.com/yuin/gopher-lua"
)

type Table struct {
	Data map[string]interface{}
}

type List struct {
	Data []interface{}
}

type NativeValue = glua.LValue
type NativeNumber = glua.LNumber
type NativeString = glua.LString
type NativeBool = glua.LBool
type NativeUserData = glua.LUserData
type NativeTable = glua.LTable

func ParseToTable(k, v NativeValue, acc map[string]interface{}) {
	switch v.Type() {
	case glua.LTString:
		acc[k.String()] = v.String()
	case glua.LTBool:
		acc[k.String()] = glua.LVAsBool(v)
	case glua.LTNumber:
		f := float64(v.(NativeNumber))
		if f == float64(int64(v.(NativeNumber))) {
			acc[k.String()] = int(v.(NativeNumber))
		} else {
			acc[k.String()] = f
		}
	case glua.LTUserData:
		userV := v.(*NativeUserData)
		if userV.Value == nil {
			acc[k.String()] = nil
		} else {
			switch v := userV.Value.(type) {
			case *Table:
				acc[k.String()] = v.Data
			case *List:
				acc[k.String()] = v.Data
			}
		}
	case glua.LTTable:
		res := map[string]interface{}{}
		v.(*NativeTable).ForEach(func(k, v NativeValue) {
			ParseToTable(k, v, res)
		})
		// Check if all the keys are integers and convert to array
		acc[k.String()] = res
		t, err := tryConvertToArray(res)
		if err == nil {
			acc[k.String()] = t
		}
	}
}

func MapNativeTable(t *NativeTable) (map[string]interface{}, []interface{}) {
	res := map[string]interface{}{}
	t.ForEach(func(k, v NativeValue) {
		ParseToTable(k, v, res)
	})

	// Check if all the keys are integers and convert to array
	at, err := tryConvertToArray(res)
	if err == nil {
		return nil, at
	}
	return res, nil
}

func tryConvertToArray(input map[string]interface{}) ([]interface{}, error) {
	keys := make([]int, 0, len(input))
	values := map[int]interface{}{}

	for k, v := range input {
		ik, err := strconv.Atoi(k)
		if err != nil {
			return nil, errors.New("non-integer key, cannot convert")
		}
		keys = append(keys, ik)
		values[ik] = v
	}

	sort.Ints(keys)

	var result []interface{}
	for _, k := range keys {
		result = append(result, values[k])
	}

	return result, nil
}
