package lua

import (
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync"

	glua "github.com/yuin/gopher-lua"
)

type Table struct {
	Data map[string]interface{}
}

func NewTableFromStringMap(input map[string]string) *Table {
	data := make(map[string]interface{})
	for k, v := range input {
		data[k] = v
	}
	return &Table{Data: data}
}

func NewTableFromStringSliceMap(input map[string][]string) *Table {
	data := make(map[string]interface{})
	for k, v := range input {
		var list []interface{}
		for i := range v {
			list = append(list, v[i])
		}
		data[k] = list
	}
	return &Table{Data: data}
}

type List struct {
	Data []interface{}
}

type HttpResponse struct {
	Once *sync.Once
	R    *http.Response
	body string
}

func (h *HttpResponse) Close() {
	if h == nil || h.R == nil || h.R.Body == nil {
		return
	}

	h.R.Body.Close()
	h.R.Body = nil
}

func (h *HttpResponse) Body() string {
	h.Once.Do(func() {
		b, _ := io.ReadAll(h.R.Body)
		h.Close()
		h.body = string(b)
	})
	return h.body
}

func (h *HttpResponse) Header(k string) string {
	return h.R.Header.Get(k)
}

func (h *HttpResponse) Headers(k string) []string {
	return h.R.Header.Values(k)
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
		// Check if all the keys are integers and convert to slice
		acc[k.String()] = res
		t, err := tryConvertToSlice(res)
		if err == nil {
			acc[k.String()] = t
		}
	}
}

func MapNativeTable(t *NativeTable) (interface{}, bool) {
	res := map[string]interface{}{}
	t.ForEach(func(k, v NativeValue) {
		ParseToTable(k, v, res)
	})

	// Check if all the keys are integers and convert to slice
	at, err := tryConvertToSlice(res)
	if err == nil {
		return at, true
	}
	return res, false
}

func tryConvertToSlice(input map[string]interface{}) ([]interface{}, error) {
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
