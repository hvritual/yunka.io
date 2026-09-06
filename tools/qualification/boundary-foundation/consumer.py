"""Inspect pinned consumer sources; exercise synthetic intent only in a disposable copy."""
from __future__ import annotations
import hashlib
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

new, old, framework, consumer, evidence = map(lambda p: Path(p).resolve(), sys.argv[1:])
evidence.mkdir(parents=True, exist_ok=True)

def tree(root: Path):
    return {p.relative_to(root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob('*')) if p.is_file() and '.git' not in p.relative_to(root).parts}

def git(root: Path, *args: str):
    return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()

def call(binary: Path, args: list[str], name: str, success: bool = True):
    result = subprocess.run([str(binary), *args], text=True, capture_output=True)
    (evidence / (name + '.stdout')).write_text(result.stdout)
    (evidence / (name + '.stderr')).write_text(result.stderr)
    assert (result.returncode == 0) == success, (name, result.returncode, result.stdout, result.stderr)
    records.append({'name': name, 'exitCode': result.returncode})
    return json.loads(result.stdout) if result.stdout.strip().startswith('{') else None

assert git(consumer, 'rev-parse', 'HEAD') == '69518dec46bdfaf45cb84a0ee25d64c132b26fc9'
before = tree(consumer)
status = git(consumer, 'status', '--porcelain')
project = consumer / 'backend-yunka'
plan_bytes = (project / 'contracts/generated/operation-plans.json').read_bytes()
plans = json.loads(plan_bytes)
applications = sorted({p['domain'] + '/' + p['application'] for p in plans['operations']})
assert len(applications) == 1
application = applications[0]
records = []
args = ['boundary', 'inspect', '--root', str(project), '--proto-path', str(framework/'contracts/proto'), '--format', 'json', application]
# The old CLI may print usage with exit 0. The missing structured capability,
# not an invented exit-status promise, is the RED condition.
red = subprocess.run([str(old), *args], text=True, capture_output=True)
(evidence/'before-RED.stdout').write_text(red.stdout)
(evidence/'before-RED.stderr').write_text(red.stderr)
try:
    old_value = json.loads(red.stdout)
except json.JSONDecodeError:
    old_value = None
assert not isinstance(old_value, dict) or 'fingerprintDigest' not in old_value
legacy = call(new, args, 'legacy-inspect')
assert legacy['authority'] == 'read_only'
assert legacy['intentCoverage']['state'] == 'unknown'
ids = sorted(p['operationId'] for p in plans['operations'])
assert len(ids) == 25
assert sorted(o['plan']['operationId'] for o in legacy['fingerprint']['operations']) == ids
assert sorted(legacy['intentCoverage']['unknownOperations']) == ids
assert legacy == call(new, args, 'legacy-repeat')
assert (evidence/'legacy-inspect.stdout').read_bytes() == (evidence/'legacy-repeat.stdout').read_bytes()
assert all((project/s['projectPath']).is_file() for s in legacy['sources'])
assert not any(k in legacy for k in ('outcome', 'boundaryDecision', 'mutationAuthorized'))
call(new, args[:-1]+['not/a-real-application'], 'unknown-application', False)

with tempfile.TemporaryDirectory(prefix='yunka-boundary-consumer-') as tmp:
    root = Path(tmp)/'backend-yunka'
    shutil.copytree(project, root, ignore=shutil.ignore_patterns('.git'))
    include = framework/'contracts/proto'
    clone_args = ['boundary', 'inspect', '--root', str(root), '--proto-path', str(include), '--format', 'json', application]
    copied = call(new, clone_args, 'copy-before')
    assert copied['fingerprintDigest'] == legacy['fingerprintDigest'], 'absolute checkout path leaked into fingerprint'
    module_match = re.search(r'^module\s+(\S+)', (root/'go.mod').read_text(), re.M)
    assert module_match
    module = module_match.group(1)

    def artifacts(label):
        out = Path(tmp)/label
        opts = ['--proto-dir',str(root/'contracts/proto'),'--proto-path',str(include),'--out',str(out/'contract'),
                '--application-out',str(out/'application'),'--application-import',module+'/internal','--title','boundary qualification','--version','1.0.0']
        call(new, ['contract','generate',*opts], label+'-generate')
        call(new, ['contract','check',*opts], label+'-check')
        return out

    before_artifacts = artifacts('before-artifacts')
    assert (before_artifacts/'contract/operation-plans.json').read_bytes() == plan_bytes
    target = next(o for o in copied['fingerprint']['operations'] if o['plan']['bindings'].get('rpc'))
    operation = target['plan']['operationId']
    service_path = next(s['projectPath'] for s in copied['sources'] if s['canonical'] == copied['fingerprint']['serviceSource'])
    path = root/service_path
    source = path.read_text()
    pattern = re.compile(r'option\s*\(\s*yunka\.dsl\.v1\.operation\s*\)\s*=\s*\{')
    matches = list(pattern.finditer(source))
    insertion = []
    for i, match in enumerate(matches):
        stop = matches[i+1].start() if i+1 < len(matches) else len(source)
        op_id = re.search(r'\bid\s*:\s*"([^"]+)"',source[match.end():stop])
        if op_id and op_id.group(1) == operation:
            insertion.append(match.end())
    assert len(insertion) == 1, (operation, insertion)
    point = insertion[0]
    intent = '\n boundary: { context: "qualification.boundary" aggregate_not_applicable_reason: "Synthetic metadata round-trip only; not business-boundary approval" }\n'
    path.write_text(source[:point]+intent+source[point:])
    declared = call(new, clone_args, 'synthetic-intent')
    assert declared['intentCoverage']['state'] == 'partial'
    assert declared['intentCoverage']['declaredOperations'] == [operation]
    assert len(declared['intentCoverage']['unknownOperations']) == 24
    assert declared['fingerprintDigest'] != legacy['fingerprintDigest']
    assert declared['operationPlansDigest'] == legacy['operationPlansDigest']
    after_artifacts = artifacts('after-artifacts')
    for artifact in ('operation-plans.json','openapi.json','client.ts'):
        assert (before_artifacts/'contract'/artifact).read_bytes() == (after_artifacts/'contract'/artifact).read_bytes(), artifact
    assert tree(before_artifacts/'application') == tree(after_artifacts/'application'), 'execution code changed'
    manifest = json.loads((after_artifacts/'contract/manifest.json').read_text())
    assert manifest['schemaVersion'] == 5
    assert any(m.get('operation',{}).get('boundary',{}).get('context') == 'qualification.boundary'
               for service in manifest['services'] for m in service['methods'])
    # Existing generated files cannot mask a broken canonical source.
    path.write_text('invalid protobuf source')
    call(new, clone_args, 'broken-source', False)
    report = {'consumer':git(consumer,'rev-parse','HEAD'),'application':application,'operationCount':len(ids),
              'legacyIntent':'unknown','syntheticOperation':operation,'declaredCount':1,'unknownCount':24,
              'beforeFingerprint':legacy['fingerprintDigest'],'afterFingerprint':declared['fingerprintDigest'],
              'executionPlansUnchanged':True,'openAPIAndClientUnchanged':True,'generatedApplicationUnchanged':True,
              'originalConsumerUnchanged':True,'checks':records,
              'scope':'read-only canonical inspection and synthetic intent round-trip, not boundary validity, growth enforcement or consumer runtime qualification'}
    (evidence/'consumer-result.json').write_text(json.dumps(report,indent=2)+'\n')
assert before == tree(consumer)
assert status == git(consumer,'status','--porcelain')
assert plan_bytes == (project/'contracts/generated/operation-plans.json').read_bytes()
print(json.dumps(report,indent=2))
