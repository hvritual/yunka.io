package domain

import (
	"fmt"
	"strings"
)

func writePolicyRESTObject(b *strings.Builder, object ObjectSpec) {
	entity := objectGoName(object)
	plural := exportedIdentifier(pluralize(object.Name))
	fmt.Fprintf(b, "type create%sRequest struct{\n", entity)
	writeMultiFields(b, object.Fields)
	b.WriteString("}\n")
	fmt.Fprintf(b, "type update%sRequest struct{Version uint64 `json:\"version\"`\n", entity)
	writeMultiFields(b, object.Fields)
	b.WriteString("}\n")
	fmt.Fprintf(b, "func create%s(w http.ResponseWriter,r *http.Request,service application.Service){var wire create%sRequest;if err:=json.NewDecoder(r.Body).Decode(&wire);err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return};output,err:=service.Create%s(r.Context(),application.Create%sInput{", entity, entity, entity, entity)
	for _, field := range object.Fields {
		name := fieldGoName(field)
		fmt.Fprintf(b, "%s:wire.%s,", name, name)
	}
	b.WriteString("});writeJSON(w,output,err)}\n")
	fmt.Fprintf(b, "func get%s(w http.ResponseWriter,r *http.Request,service application.Service){output,err:=service.Get%s(r.Context(),application.Get%sInput{ID:r.PathValue(\"id\")});writeJSON(w,output,err)}\n", entity, entity, entity)
	fmt.Fprintf(b, "func list%s(w http.ResponseWriter,r *http.Request,service application.Service){limit,_:=strconv.Atoi(r.URL.Query().Get(\"limit\"));offset,_:=strconv.Atoi(r.URL.Query().Get(\"offset\"));output,err:=service.List%s(r.Context(),application.List%sInput{Limit:limit,Offset:offset});writeJSON(w,output,err)}\n", entity, plural, plural)
	fmt.Fprintf(b, "func update%s(w http.ResponseWriter,r *http.Request,service application.Service){var wire update%sRequest;if err:=json.NewDecoder(r.Body).Decode(&wire);err!=nil{http.Error(w,err.Error(),http.StatusBadRequest);return};output,err:=service.Update%s(r.Context(),application.Update%sInput{ID:r.PathValue(\"id\"),Version:wire.Version,", entity, entity, entity, entity)
	for _, field := range object.Fields {
		name := fieldGoName(field)
		fmt.Fprintf(b, "%s:wire.%s,", name, name)
	}
	b.WriteString("});writeJSON(w,output,err)}\n")
	fmt.Fprintf(b, "func delete%s(w http.ResponseWriter,r *http.Request,service application.Service){version,_:=strconv.ParseUint(r.URL.Query().Get(\"version\"),10,64);if err:=service.Delete%s(r.Context(),application.Delete%sInput{ID:r.PathValue(\"id\"),Version:version});err!=nil{writeError(w,err);return};w.WriteHeader(http.StatusNoContent)}\n", entity, entity, entity)
}
