package ini4go

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	kNoTag = "-"
	kTag   = "ini"
)

func (this *Ini) Unmarshal(v interface{}) error {
	return unmarshal(this, v)
}

func (this *Ini) UnmarshalSection(section string, v interface{}) error {
	var ns = this.Section(section)
	return UnmarshalSection(ns, v)
}

func Unmarshal(data []byte, v interface{}) error {
	var ini = New(false)
	if err := ini.load(bytes.NewReader(data)); err != nil {
		return err
	}
	return unmarshal(ini, v)
}

func unmarshal(ini *Ini, v interface{}) error {
	var vType = reflect.TypeOf(v)
	var vValue = reflect.ValueOf(v)
	var vValueKind = vValue.Kind()

	if vValueKind == reflect.Struct {
		return errors.New("v argument is struct")
	}

	if vValue.IsNil() {
		return errors.New("v argument is nil")
	}

	for {
		if vValueKind == reflect.Ptr && vValue.IsNil() {
			vValue.Set(reflect.New(vType.Elem()))
		}

		if vValueKind == reflect.Ptr {
			vValue = vValue.Elem()
			vType = vType.Elem()
			vValueKind = vValue.Kind()
			continue
		}
		break
	}
	return unmarshalSections(vType, vValue, ini)
}

func unmarshalSections(objType reflect.Type, objValue reflect.Value, ini *Ini) error {
	var numField = objType.NumField()
	for i := 0; i < numField; i++ {
		var fieldStruct = objType.Field(i)
		var fieldValue = objValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		var tag = fieldStruct.Tag.Get(kTag)
		if tag == "" {
			tag = fieldStruct.Name
		} else if tag == kNoTag {
			continue
		}

		var section = ini.section(tag)
		if section == nil {
			continue
		}

		if fieldValue.Kind() == reflect.Ptr {
			if fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			}
			fieldValue = fieldValue.Elem()
		}

		if fieldValue.Kind() == reflect.Struct {
			if err := unmarshalOptions(fieldValue.Addr().Type().Elem(), fieldValue, section); err != nil {
				return err
			}
		}
	}
	return nil
}

func UnmarshalSection(section *Section, v interface{}) error {
	if section == nil {
		return fmt.Errorf("section not exists")
	}

	var vType = reflect.TypeOf(v)
	var vValue = reflect.ValueOf(v)
	var vValueKind = vValue.Kind()

	if vValueKind == reflect.Struct {
		return errors.New("v argument is struct")
	}

	if vValue.IsNil() {
		return errors.New("v argument is nil")
	}

	for {
		if vValueKind == reflect.Ptr && vValue.IsNil() {
			vValue.Set(reflect.New(vType.Elem()))
		}

		if vValueKind == reflect.Ptr {
			vValue = vValue.Elem()
			vType = vType.Elem()
			vValueKind = vValue.Kind()
			continue
		}
		break
	}
	return unmarshalSection(vType, vValue, section)
}

func unmarshalSection(objType reflect.Type, objValue reflect.Value, section *Section) error {
	return unmarshalOptions(objType, objValue, section)
}

func unmarshalOptions(objType reflect.Type, objValue reflect.Value, section *Section) error {
	var numField = objType.NumField()
	for i := 0; i < numField; i++ {
		var fieldStruct = objType.Field(i)
		var fieldValue = objValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		var tag = fieldStruct.Tag.Get(kTag)

		if tag == "" {
			tag = fieldStruct.Name
		} else if tag == kNoTag {
			continue
		}

		var option = section.Option(tag)
		if option == nil {
			continue
		}

		if err := setValue(fieldValue, fieldStruct, option.Values()); err != nil {
			return err
		}
	}
	return nil
}

func setValue(fieldValue reflect.Value, fieldStruct reflect.StructField, value interface{}) error {
	var vValue = reflect.ValueOf(value)
	var fieldValueKind = fieldValue.Kind()

	if fieldValueKind == reflect.Slice {
		var valueLen int
		if vValue.Kind() == reflect.Slice {
			// 如果绑定源数据也是 slice
			valueLen = vValue.Len()
			var s = reflect.MakeSlice(fieldValue.Type(), valueLen, valueLen)
			for i := 0; i < valueLen; i++ {
				if err := _setValue(s.Index(i), fieldStruct, vValue.Index(i)); err != nil {
					return err
				}
			}
			fieldValue.Set(s)
		} else {
			// 如果绑定源数据不是 slice
			valueLen = 1
			var s = reflect.MakeSlice(fieldValue.Type(), valueLen, valueLen)
			if err := _setValue(s.Index(0), fieldStruct, vValue); err != nil {
				return err
			}
			fieldValue.Set(s)
		}
	} else {
		return _setValue(fieldValue, fieldStruct, vValue)
	}
	return nil
}

func _setValue(fieldValue reflect.Value, fieldStruct reflect.StructField, value reflect.Value) error {
	var valueKind = value.Kind()
	var fieldKind = fieldValue.Kind()

	if valueKind == reflect.Slice {
		// 如果源数据是 slice, 则取出其第一个数据
		value = value.Index(0)
		valueKind = value.Kind()
	}

	if valueKind == fieldKind {
		return _setValueWithSameKind(fieldValue, fieldStruct, valueKind, value)
	}
	return _setValueWithDiffKind(fieldValue, fieldStruct, valueKind, value)
}

func _setValueWithSameKind(fieldValue reflect.Value, fieldStruct reflect.StructField, valueKind reflect.Kind, value reflect.Value) error {
	switch valueKind {
	case reflect.String:
		fieldValue.SetString(value.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fieldValue.SetInt(value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		fieldValue.SetUint(value.Uint())
	case reflect.Float32, reflect.Float64:
		fieldValue.SetFloat(value.Float())
	case reflect.Bool:
		fieldValue.SetBool(value.Bool())
	case reflect.Struct:
		fieldValue.Set(value)
	default:
		return errors.New(fmt.Sprintf("Unknown type: %s", fieldStruct.Name))
	}
	return nil
}

func _setValueWithDiffKind(fieldValue reflect.Value, fieldStruct reflect.StructField, valueKind reflect.Kind, value reflect.Value) (err error) {
	var fieldValueKind = fieldValue.Kind()

	switch fieldValueKind {
	case reflect.String:
		fieldValue.SetString(stringValue(valueKind, value))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fieldValue.SetInt(intValue(valueKind, value))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		fieldValue.SetUint(uintValue(valueKind, value))
	case reflect.Float32, reflect.Float64:
		fieldValue.SetFloat(floatValue(valueKind, value))
	case reflect.Bool:
		fieldValue.SetBool(boolValue(valueKind, value))
	default:
		return errors.New(fmt.Sprintf("Unknown type: %s", fieldStruct.Name))
	}
	return nil
}

func boolValue(valueKind reflect.Kind, value reflect.Value) bool {
	switch valueKind {
	case reflect.String:
		var v = value.String()
		if v == "true" || v == "yes" || v == "on" || v == "t" || v == "y" || v == "1" {
			return true
		}
		return false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Int() == 1 {
			return true
		}
		return false
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if value.Uint() == 1 {
			return true
		}
		return false
	case reflect.Float32, reflect.Float64:
		if value.Float() > 0.9990 {
			return true
		}
		return false
	case reflect.Bool:
		return value.Bool()
	}
	return false
}

func stringValue(valueKind reflect.Kind, value reflect.Value) string {
	switch valueKind {
	case reflect.Bool:
		return strconv.FormatBool(value.Bool())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(value.Uint(), 10)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	case reflect.Float32:
		return strconv.FormatFloat(value.Float(), 'f', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64)
	case reflect.String:
		return value.String()
	}
	return fmt.Sprintf("%v", value)
}

func floatValue(valueKind reflect.Kind, value reflect.Value) float64 {
	switch valueKind {
	case reflect.String:
		var sv = value.String()
		if sv == "" {
			sv = "0"
		}
		var v, e = strconv.ParseFloat(sv, 64)
		if e != nil {
			return 0
		}
		return v
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(value.Uint())
	case reflect.Float32, reflect.Float64:
		return value.Float()
	case reflect.Bool:
		var b = value.Bool()
		if b {
			return 1.0
		}
		return 0.0
	}
	return 0.0
}

func intValue(valueKind reflect.Kind, value reflect.Value) int64 {
	switch valueKind {
	case reflect.Bool:
		var v = value.Bool()
		if v {
			return 1
		}
		return 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(value.Uint())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Float32, reflect.Float64:
		return int64(value.Float())
	case reflect.String:
		var vList = strings.Split(value.String(), ".")
		var f, err = strconv.ParseInt(vList[0], 10, 64)
		if err == nil {
			return f
		}
	}
	return 0.0
}

func uintValue(valueKind reflect.Kind, value reflect.Value) uint64 {
	switch valueKind {
	case reflect.Bool:
		var v = value.Bool()
		if v {
			return 1
		}
		return 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(value.Int())
	case reflect.Float32, reflect.Float64:
		return uint64(value.Float())
	case reflect.String:
		var vList = strings.Split(value.String(), ".")
		var f, err = strconv.ParseUint(vList[0], 10, 64)
		if err == nil {
			return f
		}
	}
	return 0.0
}
