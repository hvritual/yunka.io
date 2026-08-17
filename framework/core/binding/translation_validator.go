package binding

import (
	"errors"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"gopkg.in/go-playground/validator.v9"
	zh_translation "gopkg.in/go-playground/validator.v9/translations/zh"
	"reflect"
	"sync"
)

type translationValidator struct {
	validate   *validator.Validate
	once       sync.Once
	translator ut.Translator
}

func (v *translationValidator) ValidateStruct(obj interface{}) error {
	value := reflect.ValueOf(obj)
	valueType := value.Kind()
	if valueType == reflect.Ptr {
		valueType = value.Elem().Kind()
	}
	if valueType == reflect.Struct {
		v.lazyinit()
		if err := v.validate.Struct(obj); err != nil {
			translatedErrs := err.(validator.ValidationErrors).Translate(v.translator)
			// get the first error message.
			for _, errMsg := range translatedErrs {
				err = errors.New(errMsg)
				break
			}
			return err
		}
	}
	return nil
}

func (v *translationValidator) Engine() interface{} {
	v.lazyinit()
	return v.validate
}

func (v *translationValidator) lazyinit() {
	v.once.Do(func() {
		v.validate = validator.New()
		v.validate.SetTagName(`binding`)

		// register translations.
		zh_cn := zh.New()
		universalTranslator := ut.New(zh_cn, zh_cn)

		translator, _ := universalTranslator.GetTranslator(`zh`)
		v.translator = translator
		zh_translation.RegisterDefaultTranslations(v.validate, translator)

		// register a function to get alternate names for StructFields.
		v.validate.RegisterTagNameFunc(func(field reflect.StructField) string {
			locale := field.Tag.Get(`locale`)
			if locale == `` || locale == `-` {
				return ``
			}
			return locale
		})
	})
	return
}
