package main

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"

 "github.com/hvritual/yunka.io/pkg/contract"
 "yunka.io/app/cmd/boundarycore"
 "yunka.io/app/cmd/projectflow"
)

func clone(m contract.Manifest) contract.Manifest { b,e:=json.Marshal(m);must(e);var c contract.Manifest;must(json.Unmarshal(b,&c));return c }
func must(e error){if e!=nil{panic(e)}}
func save(dir,name string,v any){b,e:=json.MarshalIndent(v,"","  ");must(e);must(os.WriteFile(filepath.Join(dir,name),append(b,'\n'),0600))}
func candidate(before contract.Manifest, declared bool) contract.Manifest {
 after:=clone(before)
 for si:=range after.Services {
  s:=&after.Services[si]
  for _,m:=range clone(before).Services[si].Methods {
   if m.Operation==nil || m.Operation.ID!="delivery.dashboard.get"{continue}
   m.Name="BoundaryProbe";m.FullName=s.FullName+".BoundaryProbe"
   m.Operation.ID="qualification.boundary.probe";m.Operation.UseCase="boundary_probe"
   m.Authorization=nil;m.Directives=nil
   for hi:=range m.HTTP{m.HTTP[hi].Path=fmt.Sprintf("/qualification/boundary-probe/%d",hi)}
   if declared{m.Operation.Boundary=&contract.BoundaryIntent{Context:"qualification.context",Aggregate:"item"}}else{m.Operation.Boundary=nil}
   s.Methods=append(s.Methods,m);return after
  }
 }
 panic("pinned consumer lacks dashboard target")
}
func main(){
 if len(os.Args)!=4{panic("expected consumer-root framework-root evidence-directory")}
 root,framework,out:=os.Args[1],os.Args[2],os.Args[3]
 snapshot,e:=projectflow.DescribeContractSourceSnapshot(context.Background(),projectflow.Options{Root:filepath.Join(root,"backend-yunka"),ProtoPaths:[]string{filepath.Join(framework,"contracts/proto")}});must(e)
 before:=snapshot.Manifest
 request:=boundarycore.AdditionRequest{BaseSHA:"69518dec46bdfaf45cb84a0ee25d64c132b26fc9",Application:"delivery/management",OperationID:"qualification.boundary.probe"}
 legacy,e:=boundarycore.Inspect(before,request.Application);must(e)
 if len(legacy.Fingerprint.Operations)!=25||legacy.IntentCoverage.State!="unknown"{panic("unexpected real legacy coverage")}
 after:=candidate(before,true)
 unknown,e:=boundarycore.EvaluateAddition(request,before,after);must(e)
 if unknown.Outcome!=boundarycore.ArchitectureReviewRequired{panic("legacy boundary was silently approved")}
 must(boundarycore.RevalidateAddition(request,before,after,unknown));save(out,"consumer-legacy-decision.json",unknown)
 // This is explicitly synthetic model metadata, not a proposed business migration.
 synthetic:=clone(before)
 for si:=range synthetic.Services {
  s:=&synthetic.Services[si]
  if s.Application==nil||s.Domain+"/"+s.Application.Name!=request.Application{continue}
  for mi:=range s.Methods{if s.Methods[mi].Operation!=nil{s.Methods[mi].Operation.Boundary=&contract.BoundaryIntent{Context:"qualification.context",Aggregate:"item"}}}
  for oi:=range s.Application.Operations{s.Application.Operations[oi].Boundary=&contract.BoundaryIntent{Context:"qualification.context",Aggregate:"item"}}
 }
 annotated,e:=boundarycore.Inspect(synthetic,request.Application);must(e)
 if annotated.OperationPlansDigest!=legacy.OperationPlansDigest{panic("architecture annotations changed execution plans")}
 prospective:=candidate(synthetic,true)
 reuse,e:=boundarycore.EvaluateAddition(request,synthetic,prospective);must(e)
 if reuse.Outcome!=boundarycore.ReuseExistingApplication{panic(fmt.Sprintf("comparable synthetic peer rejected: %+v",reuse))}
 save(out,"consumer-synthetic-reuse.json",reuse)
 repeated,e:=boundarycore.EvaluateAddition(request,synthetic,prospective);must(e)
 x,_:=json.Marshal(reuse);y,_:=json.Marshal(repeated);if string(x)!=string(y){panic("nondeterministic decision")}
 must(boundarycore.RevalidateAddition(request,synthetic,prospective,reuse))
 for si:=range prospective.Services{for mi:=range prospective.Services[si].Methods{m:=&prospective.Services[si].Methods[mi];if m.Operation!=nil&&m.Operation.ID==request.OperationID{m.Operation.Boundary.Context="qualification.other"}}}
 distinct,e:=boundarycore.EvaluateAddition(request,synthetic,prospective);must(e)
 if distinct.Outcome!=boundarycore.CreateNewApplication{panic("context contradiction accepted")}
 if !errors.Is(boundarycore.RevalidateAddition(request,synthetic,prospective,reuse),boundarycore.ErrStaleBoundaryProof){panic("stale intent proof accepted")}
 save(out,"consumer-synthetic-contradiction.json",distinct)
 forged:=distinct;forged.CounterEvidence=nil
 if !errors.Is(boundarycore.RevalidateAddition(request,synthetic,prospective,forged),boundarycore.ErrStaleBoundaryProof){panic("omitted counter-evidence accepted")}
 final,e:=boundarycore.Inspect(before,request.Application);must(e)
 if final.FingerprintDigest!=legacy.FingerprintDigest{panic("mutated original manifest")}
 save(out,"consumer-result.json",map[string]any{"consumer":request.BaseSHA,"operationCount":25,"legacy":unknown.Outcome,"syntheticComparable":reuse.Outcome,"syntheticContradiction":distinct.Outcome,"staleAndTamperedRejected":true,"originalManifestUnchanged":true,"scope":"pure decision/revalidation on freshly compiled real consumer plus explicitly synthetic prospective models; not authoring gate or consumer runtime qualification"})
 fmt.Println("PASS: real unknown boundary; synthetic comparable/contradicting addition; stale/tampered proof rejection")
}
