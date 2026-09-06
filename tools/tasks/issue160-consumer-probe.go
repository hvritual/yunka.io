package main

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "reflect"
 "strings"

 "github.com/hvritual/yunka.io/pkg/contract"
 "github.com/hvritual/yunka.io/pkg/operationplan"
 "yunka.io/app/cmd/projectflow"
)

func must(err error) { if err != nil { panic(err) } }
func main() {
 root, err := filepath.Abs(os.Args[1]); must(err)
 support, err := filepath.Abs(os.Args[2]); must(err)
 options := projectflow.Options{Root:root, ProtoPaths:[]string{support}}
 first, err := projectflow.DescribeOperationContractContexts(context.Background(), options); must(err)
 second, err := projectflow.DescribeOperationContractContexts(context.Background(), options); must(err)
 if !reflect.DeepEqual(first,second) || len(first) != 25 { panic(fmt.Sprintf("unstable contexts or operation count: %d",len(first))) }
 for _, value := range first {
  if len(value.SourceFiles)<2 { panic("multi-file source closure missing") }
  for _, name := range value.SourceFiles {
   if filepath.IsAbs(name) || strings.HasPrefix(name,"../") { panic("source path escapes project") }
   info, err := os.Stat(filepath.Join(root,filepath.FromSlash(name))); must(err)
   if !info.Mode().IsRegular() { panic("non-file provenance") }
  }
 }
 compiled, err := contract.Compile(context.Background(), contract.CompileOptions{Dir:filepath.Join(root,"contracts","proto"),ProtoPaths:[]string{support}}); must(err)
 for _, message := range compiled.Manifest.Messages { if message.SourceFile=="" { panic("DTO lacks provenance: "+message.FullName) } }
 plans, err := contract.CompileOperationPlans(compiled.Manifest); must(err)
 data, err := operationplan.CanonicalJSON(plans); must(err)
 previous, err := os.ReadFile(filepath.Join(root,"contracts","generated","operation-plans.json")); must(err)
 if !bytes.Equal(data,previous) { panic("canonical operation semantics drifted") }
 result := map[string]any{"consumer":"69518dec46bdfaf45cb84a0ee25d64c132b26fc9","operations":len(first),"messages":len(compiled.Manifest.Messages),"files":len(compiled.Manifest.Files),"sourcePathsExist":true,"contextDeterministic":true,"operationPlansUnchanged":true,"scope":"read-only contract/provenance, not runtime or complete issue160 qualification"}
 must(json.NewEncoder(os.Stdout).Encode(result))
}
