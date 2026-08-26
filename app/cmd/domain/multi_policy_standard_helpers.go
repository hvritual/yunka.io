package domain

import (
	"fmt"
	"strings"
)

func writeStandardPolicyHelpers(b *strings.Builder, object ObjectSpec) {
	entity := objectGoName(object)
	siteField, ownerField := policyScopeGoFields(object)

	fmt.Fprintf(b, "type %sPermissions struct { Create string; Read string; Update string; Delete string }\n", entity)
	fmt.Fprintf(b, "func Standard%sAccess(resolver policy.Resolver,permissions %sPermissions)%sPolicy{return %sAccessPolicy{Resolver:resolver,Create:policy.Permission(permissions.Create,%s),Read:policy.Permission(permissions.Read,%s),Update:policy.Permission(permissions.Update,standard%sUpdateMatcher{}),Delete:policy.Permission(permissions.Delete,%s)}}\n", entity, entity, entity, entity, standardCreateMatcherExpr(entity, siteField), standardEntityMatcherExpr(entity, siteField, ownerField), entity, standardEntityMatcherExpr(entity, siteField, ownerField))

	fmt.Fprintf(b, "type standard%sUpdateMatcher struct{}\n", entity)
	fmt.Fprintf(b, "func(standard%sUpdateMatcher)Allows(principal identity.Principal,grant policy.Grant,target %sUpdateTarget)bool{current:=%s;if !current.Allows(principal,grant,target.Current){return false};", entity, entity, standardEntityMatcherExpr(entity, siteField, ownerField))
	if siteField != "" {
		fmt.Fprintf(b, "if target.Input.%s==target.Current.%s{return true};destination:=policy.Any(policy.All[%sUpdateTarget](),policy.Site(func(value %sUpdateTarget)string{return value.Input.%s}));return destination.Allows(principal,grant,target)", siteField, siteField, entity, entity, siteField)
	} else {
		b.WriteString("return true")
	}
	b.WriteString("}\n")
	fmt.Fprintf(b, "func(standard%sUpdateMatcher)Filter(identity.Principal,policy.Grant)policy.Filter{return policy.Filter{}}\n", entity)
}

func standardCreateMatcherExpr(entity, siteField string) string {
	parts := []string{fmt.Sprintf("policy.All[Create%sInput]()", entity)}
	if siteField != "" {
		parts = append(parts, fmt.Sprintf("policy.Site(func(value Create%sInput)string{return value.%s})", entity, siteField))
	}
	return "policy.Any(" + strings.Join(parts, ",") + ")"
}

func standardEntityMatcherExpr(entity, siteField, ownerField string) string {
	parts := []string{fmt.Sprintf("policy.All[domain.%s]()", entity)}
	if siteField != "" {
		parts = append(parts, fmt.Sprintf("policy.Site(func(value domain.%s)string{return value.%s})", entity, siteField))
	}
	if ownerField != "" {
		parts = append(parts, fmt.Sprintf("policy.Self(func(value domain.%s)string{return value.%s})", entity, ownerField))
	}
	return "policy.Any(" + strings.Join(parts, ",") + ")"
}

func policyScopeGoFields(object ObjectSpec) (string, string) {
	var siteField, ownerField string
	for _, field := range object.Fields {
		switch field.Column {
		case "site_id":
			siteField = fieldGoName(field)
		case "created_by", "owner_id":
			if ownerField == "" {
				ownerField = fieldGoName(field)
			}
		}
	}
	return siteField, ownerField
}
