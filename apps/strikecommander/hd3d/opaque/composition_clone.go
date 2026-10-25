package opaque

import (
	"fmt"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"reflect"
)

// typedFreezeInput copies typed captured observations only. The caller has
// already reserved the full retained charge and the shared32MiB workspace.
// It intentionally does not serialize/decode as an ownership mechanism.
func typedFreezeInput(in Input, limit uint64) (Input, uint64, error) {
	n, e := retainedSceneCharge(in)
	if e != nil {
		return Input{}, 0, e
	}
	if n > limit {
		return Input{}, 0, fmt.Errorf("typed input clone exceeds intermediate capacity")
	}
	v, e := cloneCapturedValue(reflect.ValueOf(in))
	if e != nil {
		return Input{}, 0, e
	}
	out := v.Interface().(Input)
	// canonicalScene uses nil for a zero-object observation. Preserve the legacy
	// digest convention without making another Objects backing after the clone.
	if len(out.Scene.Objects) == 0 {
		out.Scene.Objects = nil
	}
	return out, n, nil
}
func cloneCapturedValue(v reflect.Value) (reflect.Value, error) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		x, e := cloneCapturedValue(v.Elem())
		if e != nil {
			return reflect.Value{}, e
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(x)
		return out, nil
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		if v.Type().Elem().Kind() == reflect.Uint8 {
			reflect.Copy(out, v)
			return out, nil
		}
		for i := 0; i < v.Len(); i++ {
			x, e := cloneCapturedValue(v.Index(i))
			if e != nil {
				return reflect.Value{}, e
			}
			out.Index(i).Set(x)
		}
		return out, nil
	case reflect.Array:
		// The captured model schema's arrays contain values only (raw bytes,
		// integers and far pointers), never slices/maps/pointers.
		if !captureArrayValuesOnly(v.Type()) {
			return reflect.Value{}, fmt.Errorf("unsupported captured reference array")
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		return out, nil
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		it := v.MapRange()
		for it.Next() {
			k, e := cloneCapturedValue(it.Key())
			if e != nil {
				return reflect.Value{}, e
			}
			x, e := cloneCapturedValue(it.Value())
			if e != nil {
				return reflect.Value{}, e
			}
			out.SetMapIndex(k, x)
		}
		return out, nil
	case reflect.Struct:
		if v.Type() == reflect.TypeFor[realspace.ObjVerdict]() {
			return reflect.Zero(v.Type()), nil
		}
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			if !out.Field(i).CanSet() {
				return reflect.Value{}, fmt.Errorf("unsupported private captured field")
			}
			x, e := cloneCapturedValue(v.Field(i))
			if e != nil {
				return reflect.Value{}, e
			}
			out.Field(i).Set(x)
		}
		return out, nil
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return v, nil
	}
	return reflect.Value{}, fmt.Errorf("unsupported captured kind %s", v.Kind())
}
func captureArrayValuesOnly(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Array:
		return captureArrayValuesOnly(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if !captureArrayValuesOnly(t.Field(i).Type) {
				return false
			}
		}
		return true
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
