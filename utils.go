package fastglue

import (
	"fmt"
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
		return nil, fmt.Errorf("Failed to encode form values to struct. Non struct type: %T", ob)
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

			scanned := false
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
					f.SetBytes(args.Peek(tag))
					continue
				}

				// Iterate through fasthttp's multiple args and assign values
				// to each item in the slice.
				for i, v := range vals {
					scanned = setVal(sl.Index(i), string(v))
				}
				f.Set(sl)
			} else {
				scanned = setVal(f, string(args.Peek(tag)))
			}

			if scanned {
				fields = append(fields, tag)
			}
		}
	}
	return fields, nil
}

func setVal(f reflect.Value, val string) bool {
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v, err := strconv.ParseInt(val, 10, 0); err == nil {
			f.SetInt(v)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v, err := strconv.ParseUint(val, 10, 0); err == nil {
			f.SetUint(v)
		}
	case reflect.Float32, reflect.Float64:
		if v, err := strconv.ParseFloat(val, 0); err == nil {
			f.SetFloat(v)
		}
	case reflect.String:
		f.SetString(val)
	case reflect.Bool:
		b, _ := strconv.ParseBool(val)
		f.SetBool(b)
	default:
		return false
	}
	return true
}
