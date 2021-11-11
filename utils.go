package fastglue

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
)

var (
	bracketSplitter = regexp.MustCompile("\\[|\\]")

	// ErrInvalidParam is returned when invalid data is provided to the ToJSON or Unmarshal function.
	// Specifically, this will be returned when there is no equals sign present in the URL query parameter.
	ErrInvalidParam = errors.New("qson: invalid url query param provided")
)

// UnmarshalArgs takes fasthttp args, converts given args to byte string and unmarshals to destination.
// Known limitation: args-to-json conversion does not take array notations into account, treats array key as string.
// eg: Legs[1]=Insights will be converted to json as follows:
// {"Legs": {"1": "Insights"}}
func UnmarshalArgs(args *fasthttp.Args, dst interface{}) error {
	b, err := toJSON(args)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, dst)
}

// toJSON will turn a query string like:
//   cat=1&bar%5Bone%5D%5Btwo%5D=2&bar[one][red]=112
// Into a JSON object with all the data merged as nicely as
// possible. Eg the example above would output:
//   {"bar":{"one":{"two":2,"red":112}}}
func toJSON(query *fasthttp.Args) ([]byte, error) {
	var (
		builder interface{} = make(map[string]interface{})
	)

	query.VisitAll(func(key, value []byte) {
		tempMap, err := queryToMap(string(key), string(value))
		if err != nil {
			return
		}
		builder = merge(builder, tempMap)
	})
	return json.Marshal(builder)
}

// queryToMap turns something like a[b][c]=4 into
//   map[string]interface{}{
//     "a": map[string]interface{}{
// 		  "b": map[string]interface{}{
// 			  "c": 4,
// 		  },
// 	  },
//   }
func queryToMap(rawKey, rawValue string) (interface{}, error) {
	pieces := bracketSplitter.Split(rawKey, -1)
	key := pieces[0]

	// If len==1 then rawKey has no [] chars and we can just
	// decode this as key=value into {key: value}
	if len(pieces) == 1 {
		var value interface{}
		// First we try parsing it as an int, bool, null, etc
		err := json.Unmarshal([]byte(rawValue), &value)
		if err != nil {
			// If we got an error we try wrapping the value in
			// quotes and processing it as a string
			err = json.Unmarshal([]byte("\""+rawValue+"\""), &value)
			if err != nil {
				// If we can't decode as a string we return the err
				return nil, err
			}
		}
		return map[string]interface{}{
			key: value,
		}, nil
	}

	// If len > 1 then we have something like a[b][c]=2
	// so we need to turn this into {"a": {"b": {"c": 2}}}
	// To do this we break our key into two pieces:
	//   a and b[c]
	// and then we set {"a": queryToMap("b[c]", value)}
	ret := make(map[string]interface{}, 0)
	var err error

	ret[key], err = queryToMap(buildNewKey(rawKey), rawValue)
	if err != nil {
		return nil, err
	}

	// When URL params have a set of empty brackets (eg a[]=1)
	// it is assumed to be an array. This will get us the
	// correct value for the array item and return it as an
	// []interface{} so that it can be merged properly.
	if pieces[1] == "" {
		temp := ret[key].(map[string]interface{})
		ret[key] = []interface{}{temp[""]}
	}
	return ret, nil
}

// buildNewKey will take something like:
// origKey = "bar[one][two]"
// pieces = [bar one two ]
// and return "one[two]"
func buildNewKey(origKey string) string {
	pieces := bracketSplitter.Split(origKey, -1)
	ret := origKey[len(pieces[0])+1:]
	ret = ret[:len(pieces[1])] + ret[len(pieces[1])+1:]
	return ret
}

// splitKeyAndValue splits a URL param at the last equal
// sign and returns the two strings. If no equal sign is
// found, the ErrInvalidParam error is returned.
func splitKeyAndValue(param string) (string, string, error) {
	li := strings.LastIndex(param, "=")
	if li == -1 {
		return "", "", ErrInvalidParam
	}
	return param[:li], param[li+1:], nil
}

// merge merges a with b if they are either both slices
// or map[string]interface{} types. Otherwise it returns b.
func merge(a interface{}, b interface{}) interface{} {
	switch aT := a.(type) {
	case map[string]interface{}:
		return mergeMap(aT, b.(map[string]interface{}))
	case []interface{}:
		return mergeSlice(aT, b.([]interface{}))
	default:
		return b
	}
}

// mergeMap merges a with b, attempting to merge any nested
// values in nested maps but eventually overwriting anything
// in a that can't be merged with whatever is in b.
func mergeMap(a map[string]interface{}, b map[string]interface{}) map[string]interface{} {
	for bK, bV := range b {
		if _, ok := a[bK]; ok {
			a[bK] = merge(a[bK], bV)
		} else {
			a[bK] = bV
		}
	}
	return a
}

// mergeSlice merges a with b and returns the result.
func mergeSlice(a []interface{}, b []interface{}) []interface{} {
	a = append(a, b...)
	return a
}

// ScanArgs takes a fasthttp.Args set, takes its keys and values
// and applies them to a given struct using reflection. The field names
// are mapped to the struct fields based on a given tag tag. The field
// names that have been mapped are also return as a list. Supports string,
// bool, number types and their slices.
//
// eg:
// type Order struct {
// 	Tradingsymbol string `url:"tradingsymbol"`
// 	Tags []string `url:"tag"`
// }
func ScanArgs(args *fasthttp.Args, obj interface{}, fieldTag string) ([]string, error) {
	ob := reflect.ValueOf(obj)
	if ob.Kind() == reflect.Ptr {
		ob = ob.Elem()
	}

	if ob.Kind() != reflect.Struct {
		return nil, fmt.Errorf("failed to decode form values to struct, received non struct type: %T", ob)
	}

	// Go through every field in the struct and look for it in the Args map.
	var fields []string
	for i := 0; i < ob.NumField(); i++ {
		f := ob.Field(i)
		if f.IsValid() && f.CanSet() {
			tag := ob.Type().Field(i).Tag.Get(fieldTag)
			if tag == "" || tag == "-" {
				continue
			}

			// Got a struct field with a tag.
			// If that field exists in the arg and convert its type.
			// Tags are of the type `tagname,attribute`
			tag = strings.Split(tag, ",")[0]
			if !args.Has(tag) {
				continue
			}

			var (
				scanned bool
				err     error
			)
			// The struct field is a slice type.
			if f.Kind() == reflect.Slice {
				var (
					vals    = args.PeekMulti(tag)
					numVals = len(vals)
				)

				// Make a slice.
				sl := reflect.MakeSlice(f.Type(), numVals, numVals)

				// If it's a []byte slice (=[]uint8), assign here.
				if f.Type().Elem().Kind() == reflect.Uint8 {
					br := args.Peek(tag)
					b := make([]byte, len(br))
					copy(b, br)
					f.SetBytes(b)
					continue
				}

				// Iterate through fasthttp's multiple args and assign values
				// to each item in the slice.
				for i, v := range vals {
					scanned, err = setVal(sl.Index(i), string(v))
					if err != nil {
						return nil, fmt.Errorf("failed to decode `%v`, got: `%s` (%v)", tag, v, err)
					}
				}
				f.Set(sl)
			} else {
				v := string(args.Peek(tag))
				scanned, err = setVal(f, v)
				if err != nil {
					return nil, fmt.Errorf("failed to decode `%v`, got: `%s` (%v)", tag, v, err)
				}
			}

			if scanned {
				fields = append(fields, tag)
			}
		}
	}
	return fields, nil
}

func setVal(f reflect.Value, val string) (bool, error) {
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(val, 10, 0)
		if err != nil {
			return false, fmt.Errorf("expected int")
		}
		f.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(val, 10, 0)
		if err != nil {
			return false, fmt.Errorf("expected unsigned int")
		}
		f.SetUint(v)
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(val, 0)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return false, fmt.Errorf("expected decimal")
		}
		f.SetFloat(v)
	case reflect.String:
		f.SetString(val)
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false, fmt.Errorf("expected boolean")
		}
		f.SetBool(b)
	default:
		return false, nil
	}
	return true, nil
}
