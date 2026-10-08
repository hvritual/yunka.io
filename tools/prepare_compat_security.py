#!/usr/bin/env python3
"""Author a dependency-only candidate from the exact CE340 compatibility source.

Not a replacement qualification gate. It exports a proposal, never merges it.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

BASE = '06076ce2fc170c89c7356ee070d74ebf1de340cf'
MODULES = ('pkg', 'framework', 'gateway', 'app')
ALLOWED = {'go.work.sum', 'tools/dependency-policy.json'} | {
    f'{module}/{name}' for module in MODULES for name in ('go.mod', 'go.sum')
}

def run(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()

def fixed_version(path, version):
    if path == 'google.golang.org/grpc':
        if version != 'v1.82.1':
            raise ValueError(f'unexpected gRPC baseline {version}')
        return 'v1.83.1'
    if path == 'github.com/xuri/excelize/v2':
        if version != 'v2.8.0':
            raise ValueError(f'unexpected Excelize baseline {version}')
        return 'v2.11.0'
    if path == 'go.opentelemetry.io/otel' or path.startswith('go.opentelemetry.io/otel/'):
        if version == 'v1.44.0':
            return 'v1.45.0'
        if version == 'v0.20.0':
            return 'v0.21.0'
    return None

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--root', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    root, out = args.root.resolve(), args.output.resolve()
    if out == root or root in out.parents:
        raise SystemExit('evidence must be outside source')
    out.mkdir(parents=True, exist_ok=True)
    if run('git', 'rev-parse', 'HEAD', cwd=root) != BASE:
        raise SystemExit('source SHA mismatch')
    if run('git', 'status', '--porcelain', cwd=root):
        raise SystemExit('source must start clean')
    (out / 'base-sha.txt').write_text(BASE + '\n')
    run('git', 'archive', '--format=tar', f'--output={out / "base-source.tar"}', BASE, cwd=root)
    before = {path: hashlib.sha256((root / path).read_bytes()).hexdigest()
              for path in ALLOWED if (root / path).is_file()}
    changes = []
    for module in MODULES:
        spec = json.loads(run('go', 'mod', 'edit', '-json', cwd=root / module))
        for requirement in spec.get('Require', []):
            path, version = requirement['Path'], requirement['Version']
            target = fixed_version(path, version)
            if target:
                run('go', 'mod', 'edit', f'-require={path}@{target}', cwd=root / module)
                changes.append({'file': f'{module}/go.mod', 'path': path,
                                'from': version, 'to': target})
    if not any(change['path'] == 'github.com/xuri/excelize/v2' for change in changes):
        raise SystemExit('Excelize remediation missing')
    policy_path = root / 'tools/dependency-policy.json'
    policy = json.loads(policy_path.read_text())
    pins = [row for row in policy['requiredModules'] if row['path'] == 'google.golang.org/grpc']
    if len(pins) != 1 or pins[0]['version'] != 'v1.82.1':
        raise SystemExit('unexpected dependency policy baseline')
    pins[0]['version'] = 'v1.83.1'
    policy_path.write_text(json.dumps(policy, ensure_ascii=False, indent=2) + '\n')
    (out / 'requested-upgrades.json').write_text(json.dumps(changes, indent=2) + '\n')
    subprocess.run(['make', 'tidy'], cwd=root, check=True)
    # The policy checks the entire selected module graph, not just packages
    # imported by the application. Resolve its workspace sums before freezing.
    subprocess.run(['make', 'dependency-check'], cwd=root, check=True)
    changed = run('git', 'diff', '--name-only', cwd=root).splitlines()
    if not changed or set(changed) - ALLOWED:
        raise SystemExit(f'out-of-scope dependency change: {changed}')
    if run('git', 'ls-files', '--others', '--exclude-standard', cwd=root):
        raise SystemExit('unexpected untracked source')
    run('git', 'diff', '--check', cwd=root)
    subprocess.run(['git', 'add', '--', *changed], cwd=root, check=True)
    tree = run('git', 'write-tree', cwd=root)
    (out / 'proposed-tree.txt').write_text(tree + '\n')
    (out / 'dependencies.patch').write_text(run('git', 'diff', '--cached', '--binary', BASE, cwd=root) + '\n')
    manifest = {
        'baseSha': BASE, 'proposedTree': tree, 'classification': 'COMPATIBILITY_CANDIDATE',
        'status': 'PROPOSED_NOT_ACCEPTED', 'runtimeOrGeneratorSourceChanges': False,
        'files': [{'path': path, 'beforeSha256': before.get(path),
                   'afterSha256': hashlib.sha256((root / path).read_bytes()).hexdigest()}
                  for path in sorted(changed)],
    }
    (out / 'proposal.json').write_text(json.dumps(manifest, indent=2) + '\n')
    run('git', 'archive', '--format=tar', f'--output={out / "proposed-source.tar"}', tree, cwd=root)
    print(json.dumps({'status': 'PROPOSED_NOT_ACCEPTED', 'tree': tree, 'files': changed}))

if __name__ == '__main__':
    main()
