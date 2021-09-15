package fastglue

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
)

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
