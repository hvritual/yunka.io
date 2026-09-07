"""Actual CLI legacy-envelope bypass checks on disposable consumer copies."""
from __future__ import annotations
import hashlib
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

author, old, new, framework, consumer, evidence = [Path(p).resolve() for p in sys.argv[1:]]
PIN = '69518dec46bdfaf45cb84a0ee25d64c132b26fc9'
evidence.mkdir(parents=True, exist_ok=True)
records = []

def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()

def hashes(root):
    return {p.relative_to(root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob('*')) if p.is_file() and '.git' not in p.relative_to(root).parts}

def run(binary, args, name, success=True):
    result = subprocess.run([str(binary), *map(str,args)], text=True, capture_output=True)
    (evidence/(name+'.stdout')).write_text(result.stdout)
    (evidence/(name+'.stderr')).write_text(result.stderr)
    assert (result.returncode == 0) == success, (name,result.returncode,result.stdout,result.stderr)
    records.append({'name':name,'exitCode':result.returncode})
    try:
        return json.loads(result.stdout), result.stdout
    except json.JSONDecodeError:
        return None, result.stdout

assert git(consumer,'rev-parse','HEAD') == PIN
initial = hashes(consumer)
initial_status = git(consumer,'status','--porcelain')
include = framework/'contracts/proto'
project = consumer/'backend-yunka'
original_plans = (project/'contracts/generated/operation-plans.json').read_bytes()
inspection,_ = run(new,['boundary','inspect','--root',project,'--proto-path',include,'--format','json','delivery/management'],'consumer-inspect')
assert inspection['intentCoverage']['state'] == 'unknown'
assert len(inspection['fingerprint']['operations']) == 25
source = next(s['projectPath'] for s in inspection['sources'] if s['canonical'] == inspection['fingerprint']['serviceSource'])

with tempfile.TemporaryDirectory(prefix='yunka-set-proof-consumer-') as temp:
    temporary = Path(temp)
    copied = temporary/'consumer'
    shutil.copytree(consumer,copied,symlinks=True)
    root = copied/'backend-yunka'
    args = ['add','operation','--root',root,'--source',source,
            '--use-case','qualification_boundary_probe','--rpc-name','QualificationBoundaryProbe',
            '--access','public','--tenant','optional','--transaction','none',
            '--idempotency','none','--composition','none','--format','agent-json']
    ids = ['delivery/management','delivery.qualification.boundary-probe']
    plan, plan_text = run(author,[*args,'--plan',*ids],'legacy-create-plan')
    assert plan['schemaVersion'] == 1 and 'boundaryDecision' not in plan
    run(author,[*args,*ids],'legacy-authoring')
    module = re.search(r'^module\s+(\S+)',(root/'go.mod').read_text(),re.M).group(1)
    # Generate via the public compiler; do not hand-edit derived artifacts.
    generate = ['contract','generate','--proto-dir',root/'contracts/proto',
                '--proto-path',include,'--out',root/'contracts/generated',
                '--application-out',temporary/'application-generated',
                '--application-import',module+'/internal','--title','boundary proof qualification','--version','1.0.0']
    run(new,generate,'legacy-canonical-generate')
    actual = json.loads((root/'contracts/generated/operation-plans.json').read_text())
    assert len(actual['operations']) == 26
    identity = plan['identity']
    semantics = plan['explicitSemantics']
    effects = plan.get('generatedEffects',[])
    create = {
        'operation':{'operationId':identity['operationId'],'domain':identity['domain'],'application':identity['application']},
        'planDigest':hashlib.sha256(plan_text.encode()).hexdigest(),
        'expected':{'service':identity['service'],'rpc':identity['rpc'],'requestType':identity['requestType'],
                    'responseType':identity['responseType'],'semantics':semantics},
        'editablePaths':sorted(m['path'] for m in plan['mutations']),
        'generatedPaths':sorted({e['path'] for e in effects if e.get('path')}),
        'generatedScopes':sorted({e['scope'] for e in effects if e.get('scope')})}
    # Deliberately reproduce a proofless v2 create envelope, not approved intent.
    change = {'schemaVersion':2,'baseSha':PIN,'subjects':[{'kind':'create_operation','create':create}]}
    path = temporary/'legacy-set.json'
    path.write_text(json.dumps(change,indent=2)+'\n')
    check = ['change','set','check','--root',root,'--set',path,'--proto-path',include,'--format','agent-json']
    before_check = hashes(copied)
    status = git(copied,'status','--porcelain')
    old_result,_ = run(old,check,'old-legacy-set-RED')
    assert old_result['conformant'],old_result
    _,new_text = run(new,check,'new-legacy-set-GREEN',False)
    assert 'STALE_BOUNDARY_PROOF' in new_text or 'STALE_BOUNDARY_PROOF' in (evidence/'new-legacy-set-GREEN.stderr').read_text()
    assert before_check == hashes(copied) and status == git(copied,'status','--porcelain')
    change['schemaVersion'] = 3
    path.write_text(json.dumps(change,indent=2)+'\n')
    _,upgraded_text = run(new,check,'new-schema-without-proof-GREEN',False)
    assert 'STALE_BOUNDARY_PROOF' in upgraded_text or 'STALE_BOUNDARY_PROOF' in (evidence/'new-schema-without-proof-GREEN.stderr').read_text()
    assert before_check == hashes(copied) and status == git(copied,'status','--porcelain')
    summary = {'consumer':PIN,'application':'delivery/management','baselineOperations':25,'temporaryOperations':26,
               'oldProoflessCreateSetAccepted':True,'newLegacyAndRelabeledProoflessSetsRejected':True,
               'readonlyChecksPreservedCopy':True,'originalConsumerUnchanged':True,'originalFileCount':len(initial),
               'scope':'actual CLI legacy-envelope bypass prevention on a disposable consumer copy; not business-boundary approval, consumer migration or runtime qualification',
               'checks':records}
assert initial == hashes(consumer)
assert initial_status == git(consumer,'status','--porcelain')
assert original_plans == (project/'contracts/generated/operation-plans.json').read_bytes()
(evidence/'consumer-result.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary,indent=2))
