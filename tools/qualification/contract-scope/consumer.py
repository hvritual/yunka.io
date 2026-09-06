"""Pinned real-consumer source/CLI qualification; mutate disposable copies only."""
import hashlib
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

new, old, framework, consumer, evidence = map(Path, sys.argv[1:])
project = consumer / 'backend-yunka'
evidence.mkdir(exist_ok=True)

def digests(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob('*')) if p.is_file() and '.git' not in p.relative_to(root).parts}

def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()

before = digests(consumer)
status = git(consumer, 'status', '--porcelain')
records = []
with tempfile.TemporaryDirectory(prefix='yunka-source-scope-consumer-') as directory:
    repo = Path(directory) / 'consumer'
    shutil.copytree(consumer, repo, ignore=shutil.ignore_patterns('.git'))
    root = repo / 'backend-yunka'
    # Stage explicit real DSL compiler input in the disposable baseline only.
    support = root / 'contracts/proto/yunka/dsl/v1/options.proto'
    support_added = not support.exists()
    if support_added:
        support.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(framework / 'contracts/proto/yunka/dsl/v1/options.proto', support)
    git(repo, 'init')
    git(repo, 'config', 'user.name', 'Yunka source qualification')
    git(repo, 'config', 'user.email', 'qualification@example.invalid')
    git(repo, 'add', '-A')
    git(repo, 'commit', '-m', 'disposable pinned consumer source baseline')
    base = git(repo, 'rev-parse', 'HEAD')

    def call(binary, args, name, success=True):
        result = subprocess.run([str(binary), *args, '--root', str(root)], text=True, capture_output=True)
        (evidence / (name + '.stdout')).write_text(result.stdout)
        (evidence / (name + '.stderr')).write_text(result.stderr)
        assert (result.returncode == 0) == success, (name, result.returncode, result.stdout, result.stderr)
        records.append({'name': name, 'exitCode': result.returncode})
        return json.loads(result.stdout) if result.stdout.strip().startswith('{') else None

    all_value = call(new, ['context', '--all-operations', '--json'], 'source-context-all')
    contexts = all_value['contractContext']['operations']
    plans_bytes = (root / 'contracts/generated/operation-plans.json').read_bytes()
    plans = json.loads(plans_bytes)
    assert sorted(c['operationId'] for c in contexts) == sorted(p['operationId'] for p in plans['operations'])
    assert len(contexts) == 25 and all_value['schemaVersion'] == 6
    assert all_value == call(new, ['context', '--all-operations', '--json'], 'source-context-repeat')
    assert any(len(c['declarationFiles']) < len(c['sourceFiles']) for c in contexts)
    target = next(c for c in contexts if any(p.endswith('/common.proto') for p in c['declarationFiles']) and len(c['declarationFiles']) < len(c['sourceFiles']))
    operation = target['operationId']
    required = next(p for p in target['declarationFiles'] if p.endswith('/common.proto'))
    unrelated = sorted(set(target['sourceFiles']) - set(target['declarationFiles']))[0]
    for item in contexts:
        assert set(item['declarationFiles']) <= set(item['sourceFiles'])
        assert all((root / p).is_file() for p in item['sourceFiles'])
    plan = call(new, ['change', 'plan', '--operation', operation, '--intent', 'both', '--format', 'json'], 'exact-plan')
    assert sorted(p['path'] for p in plan['editableTargets']) == target['declarationFiles']
    call(new, ['change', 'begin', '--operation', operation, '--intent', 'both', '--path', unrelated, '--format', 'json'], 'unrelated-explicit-path', False)
    contract = call(new, ['change', 'begin', '--operation', operation, '--intent', 'both', '--format', 'json'], 'begin')['contract']
    value = call(new, ['change', 'set', 'begin', '--contract', '.git/yunka/change-contract.json', '--format', 'json'], 'set-begin')['changeSet']
    set_path = Path(git(root, 'rev-parse', '--path-format=absolute', '--git-path', 'yunka/change-set.json'))
    original_required = (root / required).read_bytes()
    original_unrelated = (root / unrelated).read_bytes()

    # Actual shared-module edit, without changing business API or runtime behavior.
    (root / required).write_bytes(original_required + b'\n// issue160 required shared source qualification\n')
    passed = call(new, ['change', 'set', 'check', '--format', 'json'], 'required-shared-green')
    assert passed['conformant'] and passed['contractSources']['baseSha'] == base
    assert passed == call(new, ['change', 'set', 'check', '--format', 'json'], 'required-shared-repeat')
    single = call(new, ['change', 'check', '--format', 'json'], 'single-shared-green')
    assert not single['violations']
    (root / required).write_bytes(original_required)

    # Co-located new unrelated DTO must not gain authority from its filename.
    (root / required).write_bytes(original_required + b'\nmessage Issue160UnrelatedProbe { string note = 1; }\n')
    denied = call(new, ['change', 'set', 'check', '--format', 'json'], 'colocated-declaration-green', False)
    assert any(v['kind'] == 'contract-declaration' for v in denied['contractSources']['violations'])
    (root / required).write_bytes(original_required)

    # Same exact saved set and Git delta: old accepts widened paths, new refuses.
    value['subjects'][0]['existing']['editablePaths'].append(unrelated)
    set_path.write_text(json.dumps(value))
    (root / unrelated).write_bytes(original_unrelated + b'\n// issue160 unrelated source qualification\n')
    red = call(old, ['change', 'set', 'check', '--format', 'json'], 'old-unrelated-RED')
    assert red['conformant'], red
    green = call(new, ['change', 'set', 'check', '--format', 'json'], 'new-unrelated-GREEN', False)
    assert not green['conformant'] and any(v['kind'] == 'contract-source' and v['path'] == unrelated for v in green['contractSources']['violations'])
    (root / unrelated).write_bytes(original_unrelated)
    assert (root / 'contracts/generated/operation-plans.json').read_bytes() == plans_bytes
    assert git(repo, 'status', '--porcelain') == ''
    report = {'consumer': '69518dec46bdfaf45cb84a0ee25d64c132b26fc9', 'operationCount': len(contexts),
              'operation': operation, 'readFiles': target['sourceFiles'], 'declarationFiles': target['declarationFiles'],
              'requiredShared': required, 'unrelated': unrelated, 'checks': records,
              'disposableDSLInputAdded': support_added, 'semanticsUnchanged': True,
              'scope': 'real consumer source/CLI, not runtime qualification'}
    (evidence / 'consumer-result.json').write_text(json.dumps(report, indent=2) + '\n')
assert before == digests(consumer)
assert status == git(consumer, 'status', '--porcelain')
print(json.dumps(report, indent=2))
