"""Read-only reverse qualification: mutate disposable checkouts only, never push."""
from pathlib import Path
import hashlib
import json
import os
import subprocess

workspace = Path(os.environ['GITHUB_WORKSPACE'])
consumer = workspace / 'consumer'
project = consumer / 'backend-yunka'
framework = workspace / 'framework'
evidence = workspace / 'evidence'
evidence.mkdir(exist_ok=True)
sequence = 0

def run(args, cwd=consumer, expected=0, name='command'):
    global sequence
    sequence += 1
    result = subprocess.run([str(x) for x in args], cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    (evidence / f'{sequence:02d}-{name}.log').write_text(result.stdout + result.stderr)
    if (expected == 0 and result.returncode != 0) or (expected != 0 and result.returncode == 0):
        print(result.stdout, result.stderr)
        raise SystemExit(f'{name}: unexpected exit {result.returncode}')
    return result.stdout

def git(*args):
    return run(['git', *args], name='git').strip()

def clean():
    status = git('status', '--porcelain', '--untracked-files=all')
    if status:
        raise SystemExit(f'Unexpected consumer mutation: {status}')

expected_consumer = '20520f69d7eaf3c27c0fb3e9d79f03b4ecb059bd'
assert git('rev-parse', 'HEAD') == expected_consumer
clean()
# An external workspace overrides only module locations, never consumer source,
# generated artifacts, third_party submodule identity, or canonical go.mod.
work = workspace / 'qualification.go.work'
lines = ['go 1.25.0', 'toolchain go1.25.13', f'use {json.dumps(str(project))}']
for module in ('framework', 'gateway', 'pkg', 'infras'):
    lines.append(f'replace github.com/hvritual/yunka.io/{module} => {json.dumps(str(framework / module))}')
work.write_text('\n'.join(lines) + '\n')
os.environ['GOWORK'] = str(work)
new = workspace / 'bin/yunka-fixed'
old = workspace / 'bin/yunka-baseline'
common = ['--root', str(project)]
generate = [new, 'generate', *common, '--proto-path', str(framework / 'contracts/proto'), '--full', '--format', 'agent-json']
run(generate, name='generate-1')
clean()
run(generate, name='generate-2')
clean()
run([new, 'check', *common, '--proto-path', str(framework / 'contracts/proto'), '--full', '--format', 'agent-json'], name='canonical-check')
clean()

# The issue's original RED object is no longer reachable. This explicitly pins
# the current real consumer rather than pretending to replay that missing SHA.
old_contract = project / '.yunka/change-contract.json'
assert not old_contract.exists()
ignored = subprocess.run(['git', 'check-ignore', '--quiet', str(old_contract)], cwd=consumer)
assert ignored.returncode == 1, 'Real consumer unexpectedly ignores historical default path'
begin_args = ['change', 'begin', *common, '--operation', 'delivery.items.update', '--intent', 'both', '--base', 'HEAD', '--path', 'contracts/proto/iot_delivery.proto', '--allow-semantic', 'permission', '--allow-semantic', 'composition', '--allow-semantic', 'dependencies', '--format', 'agent-json']
run([old, *begin_args], name='real-baseline-begin')
red = json.loads(run([old, 'change', 'check', *common, '--format', 'agent-json'], expected=1, name='real-baseline-red'))
assert len(red['violations']) == 1, red
assert red['violations'][0]['path'] == '.yunka/change-contract.json', red
assert red['violations'][0]['kind'] == 'scope', red
old_contract.unlink()
clean()

run([new, *begin_args], name='fixed-default-begin')
check_args = [new, 'change', 'check', *common, '--format', 'agent-json']
checked = json.loads(run(check_args, name='fixed-clean-check'))
assert checked['changes'] == [] and checked['violations'] == [], checked
clean()

changed = project / 'internal/delivery/application/operations.go'
original = changed.read_bytes()
rogue = project / '.yunka/issue151-undeclared.txt'
try:
    changed.write_bytes(original + b'\n// Issue 151 qualification: declared handwritten delta, no semantic change.\n')
    checked = json.loads(run(check_args, name='fixed-declared-delta-check'))
    assert checked['violations'] == [], checked
    assert len(checked['changes']) == 1, checked
    assert checked['changes'][0]['path'] == 'internal/delivery/application/operations.go', checked
    assert checked['changes'][0]['class'] == 'editable', checked
    verify_args = [new, 'change', 'verify', *common, '--proto-path', str(framework / 'contracts/proto'), '--format', 'agent-json']
    verified = json.loads(run(verify_args, name='fixed-full-verify-1'))
    attestation = verified['attestation']
    assert attestation['conformant'], attestation
    for gate in ('git-delta', 'yunka-check', 'semantic-delta', 'architecture-debt', 'go-test'):
        matches = [g for g in attestation['gates'] if g['name'] == gate]
        assert len(matches) == 1 and matches[0]['status'] == 'pass', (gate, attestation['gates'])
    path_text = run(['git', 'rev-parse', '--git-path', 'yunka/change-attestation.json'], cwd=project, name='physical-state-path').strip()
    physical = Path(path_text)
    if not physical.is_absolute():
        physical = project / physical
    first = physical.read_bytes()
    run(verify_args, name='fixed-full-verify-2')
    assert physical.read_bytes() == first, 'Non-deterministic repeat attestation'
    checked_after = json.loads(run(check_args, name='check-after-attestation'))
    assert checked_after == checked, (checked_after, checked)
    rogue.write_text('undeclared\n')
    rejected = json.loads(run(check_args, expected=1, name='undeclared-rejection'))
    assert any(v['kind'] == 'scope' and v['path'] == '.yunka/issue151-undeclared.txt' for v in rejected['violations']), rejected
    (evidence / 'attestation.json').write_bytes(first)
    digest = hashlib.sha256(first).hexdigest()
finally:
    changed.write_bytes(original)
    rogue.unlink(missing_ok=True)
clean()
run([new, *begin_args], name='next-default-begin')
clean()
assert git('rev-parse', 'HEAD') == expected_consumer
assert run(['git', 'status', '--porcelain'], cwd=framework, name='framework-final-status').strip() == ''
summary = {'issue': 151, 'frameworkCandidate': os.environ['CANDIDATE_SHA'], 'consumer': expected_consumer,
           'originalRedObjectReachable': False, 'baselineImplementationRedReproduced': True,
           'generationTwiceZeroDrift': True, 'defaultBeginCheckVerify': True, 'goTestsSkipped': False,
           'declaredHandwrittenDelta': True, 'attestationDeterministic': True,
           'undeclaredPathRejected': True, 'finalConsumerClean': True, 'attestationSHA256': digest}
(evidence / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
print(json.dumps(summary, indent=2))
print('ISSUE151_REAL_IOT_REVERSE_QUALIFICATION=PASS')
