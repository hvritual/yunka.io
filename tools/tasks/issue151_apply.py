"""One-shot, user-authorized delivery script; not part of normal CI or final PR."""
from pathlib import Path
import re
import subprocess

expected = {
    'app/cmd/change/contract.go': '7587f4f8b95cf855022b955646766834851c753f',
    'app/cmd/change/verify.go': '9fdc01ed2123f04342ef2fa0914ddcae811a771c',
    'app/cmd/change/ax7_test.go': '12a28eae60427b1a2b107f9f09562b379a3e7ffc',
}
for name, sha in expected.items():
    actual = subprocess.check_output(['git', 'hash-object', name], text=True).strip()
    if actual != sha:
        raise SystemExit(f'Unexpected source identity for {name}: {actual}')

def once(text, old, new):
    if text.count(old) != 1:
        raise SystemExit(f'Expected one exact patch site, found {text.count(old)}: {old[:100]!r}')
    return text.replace(old, new, 1)

p = Path('app/cmd/change/contract.go')
s = p.read_text()
s = once(s, '".yunka/change-contract.json"', '".git/yunka/change-contract.json"')
start = s.index('func WriteChangeContract(')
end = s.index('\nfunc LoadChangeContract(', start)
b = s[start:end]
first = b.index('\n\tif err := os.MkdirAll(')
b = b[:b.index('\n')] + '''
	path, display, err := resolveGitPrivateStatePath(root, output, DefaultChangeContractPath)
	if err != nil {
		return "", err
	}''' + b[first:]
old = '''
	relative, err := filepath.Rel(root, path)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative), nil
	}
	return filepath.ToSlash(path), nil'''
b = once(b, old, '\n\treturn display, nil')
s = s[:start] + b + s[end:]
start = s.index('func LoadChangeContract(')
end = s.index('\nfunc RenderChangeContract(', start)
b = s[start:end]
first = b.index('\n\tdata, err := os.ReadFile(path)')
b = b[:b.index('\n')] + '''
	path, _, err := resolveGitPrivateStatePath(root, input, DefaultChangeContractPath)
	if err != nil {
		return ChangeContract{}, "", err
	}''' + b[first:]
s = s[:start] + b + s[end:]
p.write_text(s)

p = Path('app/cmd/change/verify.go')
s = p.read_text()
s = once(s, '".yunka/change-attestation.json"', '".git/yunka/change-attestation.json"')
start = s.index('func WriteChangeAttestation(')
end = s.index('\nfunc RenderChangeAttestation(', start)
b = s[start:end]
first = b.index('\n\tif err := os.MkdirAll(')
b = b[:b.index('\n')] + '''
	path, display, err := resolveGitPrivateStatePath(root, output, DefaultChangeAttestationPath)
	if err != nil {
		return "", err
	}''' + b[first:]
b = once(b, old, '\n\treturn display, nil')
s = s[:start] + b + s[end:]
p.write_text(s)

p = Path('app/cmd/change/ax7_test.go')
s = p.read_text()
s = once(s, 'func TestChangeContractWriteLoadIsDeterministicAndTransient(t *testing.T) {\n\troot := t.TempDir()', 'func TestChangeContractWriteLoadIsDeterministicAndTransient(t *testing.T) {\n\troot := t.TempDir()\n\tgitPressure(t, root, "init")')
p.write_text(s)

for name in ['README.md', 'docs/STATUS.md']:
    p = Path(name)
    s = p.read_text()
    s = s.replace('.yunka/change-contract.json', '.git/yunka/change-contract.json')
    s = s.replace('.yunka/change-attestation.json', '.git/yunka/change-attestation.json')
    if name == 'docs/STATUS.md':
        s = once(s, '> Reconciled date: 2026-09-05', '> Reconciled date: 2026-09-06')
        s = once(s, '## Current pressure frontier', '''## AX7 default control-state storage — issue #151

Default single-Operation Change Contract and Change Attestation now use the same Git-private path resolver as ChangeSet/remediation. Their logical defaults are `.git/yunka/change-contract.json` and `.git/yunka/change-attestation.json`; Git resolves physical storage for ordinary checkouts, nested project roots, linked worktrees, and submodules. The default `begin -> check -> verify -> check` loop does not require `.gitignore` changes and does not add control artifacts to the source delta.

The implementation changes storage only. Git delta, ownership, placement, semantic, Audit/new-debt, and Go-test gates are not weakened; no filename or directory is exempted from source reconciliation. Explicit `--output` / `--contract` paths remain literal, including historical `.yunka/...` paths. Existing files are not moved or deleted automatically. An old active contract can be selected explicitly; source-tree exports remain subject to normal scope checks. Concurrent projects in one worktree should use distinct explicit Git-private contract/output paths rather than sharing one active default slot.

The permanent issue-151 regression removes the old pressure fixture's blanket `.yunka/` ignore rule and exercises the public default commands, full Go tests, repeatable attestation, clean source delta, native Git layouts, explicit-path compatibility, and rejection of unrelated files. Exact RED/GREEN, framework, production, and consumer qualification records belong to the issue/PR delivery evidence, not a claim of universal defect freedom.

## Current pressure frontier''')
    else:
        s += '''
### Change control-state paths

`yunka change begin`, `change check`, and `change verify` share the Git-private default contract `.git/yunka/change-contract.json`; verification writes `.git/yunka/change-attestation.json`. These are logical paths resolved through Git, including linked worktrees, submodules, and nested projects. Defaults do not require an ignore rule. Explicit `--output` and `--contract` paths remain literal; existing `.yunka/...` files are not migrated or deleted automatically. Source-tree exports still participate in normal scope reconciliation. For concurrent projects in one worktree, use distinct explicit Git-private paths. No `.yunka/**` scope exemption is introduced.
'''
    p.write_text(s)

subprocess.run(['gofmt', '-w', *expected.keys(), 'app/cmd/change/default_control_state_test.go'], check=True)
allowed = set(expected) | {'app/cmd/change/default_control_state_test.go', 'README.md', 'docs/STATUS.md'}
changed = set(subprocess.check_output(['git', 'diff', '--name-only'], text=True).splitlines())
if not changed <= allowed:
    raise SystemExit(f'Unexpected mutation paths: {changed - allowed}')
subprocess.run(['git', 'diff', '--check'], check=True)
