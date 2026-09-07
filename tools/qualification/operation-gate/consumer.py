"""Real canonical consumer: demonstrate the old write and the new no-write gate."""
from __future__ import annotations
import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

old, new, framework, consumer, evidence = [Path(p).resolve() for p in sys.argv[1:]]
evidence.mkdir(parents=True, exist_ok=True)
PIN = '69518dec46bdfaf45cb84a0ee25d64c132b26fc9'

def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()

def hashes(root):
    return {p.relative_to(root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob('*')) if p.is_file() and '.git' not in p.relative_to(root).parts}

records = []
def run(binary, args, name, success):
    result = subprocess.run([str(binary), *map(str, args)], text=True, capture_output=True)
    (evidence / (name + '.stdout')).write_text(result.stdout)
    (evidence / (name + '.stderr')).write_text(result.stderr)
    assert (result.returncode == 0) == success, (name, result.returncode, result.stdout, result.stderr)
    records.append({'name': name, 'exitCode': result.returncode})
    return json.loads(result.stdout)

assert git(consumer, 'rev-parse', 'HEAD') == PIN
before = hashes(consumer)
status = git(consumer, 'status', '--porcelain')
project = consumer / 'backend-yunka'
plan_bytes = (project / 'contracts/generated/operation-plans.json').read_bytes()
plans = json.loads(plan_bytes)
keys = {p['domain'] + '/' + p['application'] for p in plans['operations']}
assert len(keys) == 1 and len(plans['operations']) == 25
application = next(iter(keys))
include = framework / 'contracts/proto'
inspection = run(new, ['boundary', 'inspect', '--root', project, '--proto-path', include,
                       '--format', 'json', application], 'consumer-inspection', True)
assert inspection['intentCoverage']['state'] == 'unknown'
assert len(inspection['fingerprint']['operations']) == 25
source = next(s['projectPath'] for s in inspection['sources']
              if s['canonical'] == inspection['fingerprint']['serviceSource'])

with tempfile.TemporaryDirectory(prefix='yunka-gate-consumer-') as temporary:
    old_root, new_root = Path(temporary)/'old', Path(temporary)/'new'
    # Full copies preserve the real Git baseline but never push or commit them.
    shutil.copytree(consumer, old_root, symlinks=True)
    shutil.copytree(consumer, new_root, symlinks=True)
    def request(root, *, modern=False, plan=False, declared=False):
        args = ['add', 'operation', '--root', root/'backend-yunka', '--source', source,
                '--use-case', 'qualification_boundary_probe', '--rpc-name', 'QualificationBoundaryProbe',
                '--access', 'public', '--tenant', 'optional', '--transaction', 'none',
                '--idempotency', 'none', '--composition', 'none', '--format', 'agent-json']
        if modern:
            args += ['--proto-path', include]
        if plan:
            args += ['--plan']
        if declared:
            args += ['--context', 'qualification.probe', '--aggregate', 'probe']
        return args + [application, 'delivery.qualification.boundary-probe']
    old_before = hashes(old_root)
    old_plan = run(old, request(old_root, plan=True), 'before-plan-RED', True)
    assert old_plan['schemaVersion'] == 1 and 'boundaryDecision' not in old_plan
    assert hashes(old_root) == old_before
    old_apply = run(old, request(old_root), 'before-apply-RED', True)
    assert len(old_apply['mutations']) == 2
    changed = sorted(p for p, digest in hashes(old_root).items() if old_before.get(p) != digest)
    assert len(changed) == 2
    # The old accepted source must compile: this is not a malformed-input counterexample.
    old_compiled = run(new, ['boundary', 'inspect', '--root', old_root/'backend-yunka',
                            '--proto-path', include, '--format', 'json', application],
                       'before-written-canonical', True)
    assert len(old_compiled['fingerprint']['operations']) == 26
    new_before = hashes(new_root)
    new_status = git(new_root, 'status', '--porcelain')
    for declared in (False, True):
        for plan in (True, False):
            name = 'after-' + ('declared-' if declared else 'unknown-') + ('plan' if plan else 'apply')
            report = run(new, request(new_root, modern=True, plan=plan, declared=declared), name, False)
            assert report['schemaVersion'] == 2 and report['baseSha'] == PIN
            assert report['boundaryDecision']['outcome'] == 'architecture_review_required'
            assert report['boundaryDecision']['authority'] == 'read_only'
            assert report['mutations'] == [] and not report.get('generatedEffects')
            assert len(report['inputsDigest']) == 64
            assert hashes(new_root) == new_before
            assert git(new_root, 'status', '--porcelain') == new_status
    summary = {'consumer': PIN, 'application': application, 'baselineOperationCount': 25,
               'oldAcceptedOperationCount': 26, 'oldChangedFiles': changed,
               'newPlanAndApplyBlockedWithoutWrites': True, 'declaredCandidateDoesNotBlessUnknownPeers': True,
               'originalConsumerUnchanged': True, 'inspectedFiles': len(before), 'checks': records,
               'scope': 'actual CLI authoring on disposable real-consumer copies; no business-boundary approval, full growth enforcement, original migration or runtime qualification'}
assert hashes(consumer) == before
assert git(consumer, 'status', '--porcelain') == status
assert (project/'contracts/generated/operation-plans.json').read_bytes() == plan_bytes
(evidence/'consumer-result.json').write_text(json.dumps(summary, indent=2)+'\n')
print(json.dumps(summary, indent=2))
