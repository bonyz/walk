// Copyright 2012 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	errValidationFailed = errors.New("validation failed")
)

type ErrorPresenter interface {
	PresentError(err error, widget Widget)
}

type DataBinder struct {
	dataSource                interface{}
	boundWidgets              []Widget
	properties                []Property
	property2Widget           map[Property]Widget
	property2ChangedHandle    map[Property]int
	widget2Property2Error     map[Widget]map[Property]error
	errorPresenter            ErrorPresenter
	canSubmitChangedPublisher EventPublisher
}

func NewDataBinder() *DataBinder {
	return new(DataBinder)
}

func (db *DataBinder) DataSource() interface{} {
	return db.dataSource
}

func (db *DataBinder) SetDataSource(dataSource interface{}) {
	db.dataSource = dataSource
}

func (db *DataBinder) BoundWidgets() []Widget {
	return db.boundWidgets
}

func (db *DataBinder) SetBoundWidgets(boundWidgets []Widget) {
	for prop, handle := range db.property2ChangedHandle {
		prop.Changed().Detach(handle)
	}

	db.boundWidgets = boundWidgets

	db.property2Widget = make(map[Property]Widget)
	db.property2ChangedHandle = make(map[Property]int)
	db.widget2Property2Error = make(map[Widget]map[Property]error)

	for _, widget := range boundWidgets {
		widget := widget

		for _, prop := range widget.BaseWidget().name2Property {
			prop := prop
			if _, ok := prop.Source().(string); !ok {
				continue
			}

			db.properties = append(db.properties, prop)
			db.property2Widget[prop] = widget

			db.property2ChangedHandle[prop] = prop.Changed().Attach(func() {
				db.validateProperty(prop, widget)
			})
		}
	}
}

func (db *DataBinder) validateProperty(prop Property, widget Widget) {
	validator := prop.Validator()
	if validator == nil {
		return
	}

	var changed bool
	prop2Err := db.widget2Property2Error[widget]

	err := validator.Validate(prop.Get())
	if err != nil {
		changed = len(db.widget2Property2Error) == 0

		if prop2Err == nil {
			prop2Err = make(map[Property]error)
			db.widget2Property2Error[widget] = prop2Err
		}
		prop2Err[prop] = err
	} else {
		if prop2Err == nil {
			return
		}

		delete(prop2Err, prop)

		if len(prop2Err) == 0 {
			delete(db.widget2Property2Error, widget)

			changed = len(db.widget2Property2Error) == 0
		}
	}

	if db.errorPresenter != nil {
		db.errorPresenter.PresentError(err, widget)
	}

	if changed {
		db.canSubmitChangedPublisher.Publish()
	}
}

func (db *DataBinder) ErrorPresenter() ErrorPresenter {
	return db.errorPresenter
}

func (db *DataBinder) SetErrorPresenter(ep ErrorPresenter) {
	db.errorPresenter = ep
}

func (db *DataBinder) CanSubmit() bool {
	return len(db.widget2Property2Error) == 0
}

func (db *DataBinder) CanSubmitChanged() *Event {
	return db.canSubmitChangedPublisher.Event()
}

func (db *DataBinder) Reset() error {
	return db.forEach(func(prop Property, field reflect.Value) error {
		if f64, ok := prop.Get().(float64); ok {
			switch v := field.Interface().(type) {
			case float32:
				f64 = float64(v)

			case float64:
				f64 = v

			case int:
				f64 = float64(v)

			case int8:
				f64 = float64(v)

			case int16:
				f64 = float64(v)

			case int32:
				f64 = float64(v)

			case int64:
				f64 = float64(v)

			case uint:
				f64 = float64(v)

			case uint8:
				f64 = float64(v)

			case uint16:
				f64 = float64(v)

			case uint32:
				f64 = float64(v)

			case uint64:
				f64 = float64(v)

			case uintptr:
				f64 = float64(v)

			default:
				return newError(fmt.Sprintf("Field '%s': Can't convert %s to float64.", prop.Source().(string), field.Type().Name()))
			}

			if err := prop.Set(f64); err != nil {
				return err
			}
		} else {
			if err := prop.Set(field.Interface()); err != nil {
				return err
			}
		}

		db.validateProperty(prop, db.property2Widget[prop])
		return nil
	})
}

func (db *DataBinder) Submit() error {
	if !db.CanSubmit() {
		return errValidationFailed
	}

	return db.forEach(func(prop Property, field reflect.Value) error {
		value := prop.Get()
		if value == nil {
			// This happens e.g. if CurrentIndex() of a ComboBox returns -1.
			// FIXME: Should we handle this differently?
			return nil
		}
		if err, ok := value.(error); ok {
			return err
		}

		if f64, ok := value.(float64); ok {
			switch field.Kind() {
			case reflect.Float32, reflect.Float64:
				field.SetFloat(f64)

			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				field.SetInt(int64(f64))

			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				field.SetUint(uint64(f64))

			default:
				return newError(fmt.Sprintf("Field '%s': Can't convert float64 to %s.", prop.Source().(string), field.Type().Name()))
			}

			return nil
		}

		field.Set(reflect.ValueOf(value))

		return nil
	})
}

func (db *DataBinder) forEach(f func(prop Property, field reflect.Value) error) error {
	p := reflect.ValueOf(db.dataSource)
	if p.Type().Kind() != reflect.Ptr {
		return newError("DataSource must be a pointer to a struct.")
	}

	if p.IsNil() {
		return nil
	}

	s := reflect.Indirect(p)
	if s.Type().Kind() != reflect.Struct {
		return newError("DataSource must be a pointer to a struct.")
	}

	for _, prop := range db.properties {
		path := prop.Source().(string)
		names := strings.Split(path, ".")

		p := p
		s := s

		for i, name := range names {
			field := s.FieldByName(name)
			if !field.IsValid() {
				return newError(fmt.Sprintf("Struct '%s' has no field '%s'.",
					s.Type().Name(), name))
			}

			if i == len(names)-1 {
				if err := f(prop, field); err != nil {
					return err
				}
			} else if p.Type().Kind() == reflect.Ptr {
				p = field
			} else {
				return newError("Field must be a pointer to a struct.")
			}

			if p.IsNil() {
				return newError("Pointer must not be nil.")
			}

			s = reflect.Indirect(p)
			if s.Type().Kind() != reflect.Struct {
				return newError("Pointer must point to a struct.")
			}
		}
	}

	return nil
}

func validateBindingMemberSyntax(member string) error {
	// FIXME
	return nil
}
