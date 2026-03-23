package tftype

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// IsAssignable checks if a value of valueType can satisfy constraintType.
// Returns true when: constraint is DynamicPseudoType (any), value type is
// DynamicPseudoType (unknown), types are equal, or a safe conversion exists.
// For object types, it strictly rejects extra attributes not in the constraint.
func IsAssignable(valueType, constraintType cty.Type) bool {
	errs := checkAssignable(valueType, constraintType, "")
	return len(errs) == 0
}

// ExtraAttrs returns detailed error messages for strict structural mismatches.
// Returns nil when the types are compatible.
func ExtraAttrs(valueType, constraintType cty.Type) []string {
	return checkAssignable(valueType, constraintType, "")
}

func checkAssignable(valueType, constraintType cty.Type, path string) []string {
	if constraintType == cty.DynamicPseudoType {
		return nil
	}
	if valueType == cty.DynamicPseudoType {
		return nil
	}
	if valueType.Equals(constraintType) {
		return nil
	}

	// map(object({...})) constraint with object({k1: ..., k2: ...}) value:
	// check each element's type against the map's element constraint.
	if constraintType.IsMapType() && valueType.IsObjectType() {
		elemConstraint := constraintType.ElementType()
		if elemConstraint.IsObjectType() {
			var errs []string
			for name, attrType := range valueType.AttributeTypes() {
				elemPath := joinPath(path, name)
				errs = append(errs, checkAssignable(attrType, elemConstraint, elemPath)...)
			}
			return errs
		}
	}

	// object constraint vs object value: check for extra attrs and recurse.
	if constraintType.IsObjectType() && valueType.IsObjectType() {
		constraintAttrs := constraintType.AttributeTypes()
		var errs []string
		for name, attrType := range valueType.AttributeTypes() {
			attrPath := joinPath(path, name)
			expectedType, ok := constraintAttrs[name]
			if !ok {
				errs = append(errs, fmt.Sprintf("unexpected attribute %q", attrPath))
				continue
			}
			errs = append(errs, checkAssignable(attrType, expectedType, attrPath)...)
		}
		return errs
	}

	// list/set constraint: check element types.
	if constraintType.IsListType() && valueType.IsListType() {
		return checkAssignable(valueType.ElementType(), constraintType.ElementType(), path+"[]")
	}
	if constraintType.IsSetType() && valueType.IsSetType() {
		return checkAssignable(valueType.ElementType(), constraintType.ElementType(), path+"[]")
	}

	// map constraint with map value: check element types.
	if constraintType.IsMapType() && valueType.IsMapType() {
		return checkAssignable(valueType.ElementType(), constraintType.ElementType(), path+"[]")
	}

	// tuple → list: check each element against the list's element type.
	if constraintType.IsListType() && valueType.IsTupleType() {
		elemConstraint := constraintType.ElementType()
		var errs []string
		for i, et := range valueType.TupleElementTypes() {
			errs = append(errs, checkAssignable(et, elemConstraint, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return errs
	}

	// Fall back to cty convert for primitives and other types.
	if convert.GetConversion(valueType, constraintType) != nil {
		return nil
	}

	if path == "" {
		return []string{fmt.Sprintf("type %s is not assignable to %s", valueType.FriendlyName(), constraintType.FriendlyName())}
	}
	return []string{fmt.Sprintf("%s: type %s is not assignable to %s", path, valueType.FriendlyName(), constraintType.FriendlyName())}
}

func joinPath(base, attr string) string {
	if base == "" {
		return attr
	}
	return base + "." + attr
}
