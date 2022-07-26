package mutation

import (
	"context"
	"reflect"
	"strings"

	"github.com/goharbor/harbor-operator/pkg/resources"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func GetOwnerMutation(scheme *runtime.Scheme, owner metav1.Object) resources.Mutable {
	return func(ctx context.Context, result runtime.Object) error {
		resourceMeta, ok := result.(metav1.Object)
		if !ok {
			return ErrorResourceType
		}

		err := controllerutil.SetControllerReference(owner, resourceMeta, scheme)

		return errors.Wrapf(err, "cannot set controller reference for %+v", result)
	}
}

func GetOverrideMutation(owner runtime.Object) resources.Mutable {
	var search func(reflect.Value)

	replaces := []struct {
		field []string
		value reflect.Value
	}{}

	search = func(value reflect.Value) {
		for i := 0; i < value.NumField(); i++ {
			if v, ok := value.Type().Field(i).Tag.Lookup("defaulter"); ok {
				for _, item := range strings.Split(v, ",") {
					replaces = append(replaces, struct {
						field []string
						value reflect.Value
					}{
						field: strings.Split(item, "."),
						value: value.Field(i),
					})
				}
				continue
			}
			filed := value.Field(i)
			if filed.Kind() == reflect.Ptr {
				filed = filed.Elem()
			}
			if filed.Kind() == reflect.Struct {
				search(filed)
			}
		}
	}

	search(reflect.ValueOf(owner.DeepCopyObject()).Elem())
	return func(ctx context.Context, result runtime.Object) error {
		value := reflect.ValueOf(result).Elem()
	notFound:
		for _, replace := range replaces {
			filed := value
			for _, name := range replace.field {
				if filed.Kind() == reflect.Ptr {
					filed = filed.Elem()
				}
				if filed.Kind() != reflect.Struct {
					continue notFound
				}
				filed = filed.FieldByName(strings.Title(name))
			}

			if filed.CanSet() && filed.IsZero() && filed.Type() == replace.value.Type() {
				filed.Set(replace.value)
			}
		}

		return nil
	}
}
