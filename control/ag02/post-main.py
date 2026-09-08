"""Post-integration consumer verification; never writes a product branch."""
from pathlib import Path
import collections
import json
import os
import subprocess

root = Path.cwd()
biz = root / 'consumer/biz'
runtime = root / 'consumer/yunka.io'
evidence = Path(os.environ['RUNNER_TEMP']) / 'ag02-delivery/consumer'
evidence.mkdir(parents=True, exist_ok=True)
head = 'df6cccf32fc2557f6a22a26b4a966a58b775ff27'
tree = 'a320d20874d4054ce8a0de18482aa984274548bf'


def git(*args):
    return subprocess.check_output(['git', *args], cwd=biz, text=True).strip()


def api(path, output):
    data = subprocess.check_output(['gh', 'api', path])
    (evidence / output).write_bytes(data)
    return json.loads(data)


def run(args, log, cwd=biz):
    with (evidence / log).open('w') as f:
        subprocess.run(args, cwd=cwd, stdout=f, stderr=subprocess.STDOUT, check=True, timeout=900)
    print('PASS', log, flush=True)


def verify_json(name, required, repeats=1):
    rows = [json.loads(x) for x in (evidence / name).read_text().splitlines()]
    assert not any(x.get('Action') in ('fail', 'skip') for x in rows), name
    counts = collections.Counter(x['Test'] for x in rows if x.get('Action') == 'pass' and x.get('Test'))
    assert set(counts) == set(required), (name, set(counts) ^ set(required))
    assert all(n == repeats for n in counts.values()), (name, counts)
    return dict(counts)


assert git('rev-parse', 'HEAD') == head
assert git('rev-parse', 'HEAD^{tree}') == tree
assert not git('status', '--porcelain')
assert git('ls-remote', 'origin', 'refs/heads/main').split()[0] == head
pr = api('repos/hvritual/biz/pulls/16', 'pull-request.json')
assert pr['merged'] and pr['merge_commit_sha'] == head and pr['head']['sha'] == head
review = api('repos/hvritual/biz/issues/comments/5579042518', 'review-summary.json')
assert review['user']['login'] == 'chatgpt-codex-connector[bot]'
assert 'Completed' in review['body'] and '`df6cccf`' in review['body']
comments = api('repos/hvritual/biz/issues/16/comments?per_page=100', 'review-comments.json')
positive = [x for x in comments if x['user']['login'] == 'chatgpt-codex-connector[bot]' and 'df6cccf' in x['body'] and "Didn't find any major issues" in x['body']]
reactions = api('repos/hvritual/biz/issues/16/reactions?per_page=100', 'review-reactions.json')
positive_reaction = any(x['user']['login'] == 'chatgpt-codex-connector[bot]' and x['content'] == '+1' and x['created_at'] >= '2026-09-08T04:35:03Z' for x in reactions)
assert positive or positive_reaction, 'no exact final independent review disposition'
run_ids = [34187433447, 34187433459, 34187433499, 34187433520, 34187433457, 34187433495, 34187433456, 34187433539, 34187433458]
for i in run_ids:
    r = api(f'repos/hvritual/biz/actions/runs/{i}', f'pr-run-{i}.json')
    assert r['head_sha'] == head and r['status'] == 'completed' and r['conclusion'] == 'success'

run(['make', 'consumer-certify'], 'consumer-certify.log')
run(['make', 'check'], 'check-before.log')
run(['make', 'generate'], 'generate.log')
assert not git('status', '--porcelain'), 'generation drift'
run(['make', 'check'], 'check-after.log')
run(['go', 'test', '-json', '-count=1', './internal/access/application/tenantlifecycle/...', './internal/architecture'], 'targeted.jsonl')
targets = ['TestAG02BuildExportsOnlyDeclaredApplication', 'TestAG02BuildPreservesRequiredDependencies', 'TestAG02GeneratedChildrenKeepActualMethodSetNarrow', 'TestAG02TenantFactoryIsCompositionOnly', 'TestTenantLifecycleRequiresRootExecutionScope', 'TestTenantLifecycleStateChangesUseJoinedRootUnitOfWork']
summary = {'targeted': verify_json('targeted.jsonl', targets)}
os.environ['BIZ_AG02_EVIDENCE'] = str(evidence / 'import-probes')
run(['make', 'tenant-boundary-check'], 'boundary.log')
run(['go', 'test', '-count=1', './...'], 'all-tests.log')
run(['go', 'test', '-race', '-count=1', './internal/...'], 'race.log')
run(['go', 'vet', './...'], 'vet.log')
run(['go', 'build', './...'], 'build.log')
run(['go', 'test', '-json', '-count=1', '-timeout=10m', '-tags=integration', './integration'], 'mysql.jsonl')
mysql = ['TestAG02OwnerInvariantUsesCurrentReadAfterSnapshot/revoke', 'TestAG02OwnerInvariantUsesCurrentReadAfterSnapshot/suspend', 'TestAG02OwnerInvariantUsesCurrentReadAfterSnapshot', 'TestB126ConcurrentSameEmailInviteConvergesToOneGlobalUser', 'TestB126ConcurrentPermissionReplacementHasSingleWinnerAndNoMixedSet', 'TestB126ConcurrentOwnerRevokesCannotRemoveAllOwners', 'TestB126ConcurrentTenantCreateSameIdempotencyKeyCreatesOneBootstrapTree', 'TestB123MemberDeactivationPreservesLastActiveOwner', 'TestB123TenantMemberLifecycleIsTenantScopedAcrossRESTAndGRPC', 'TestB126RoleRevokeAndMemberSuspendCannotRemoveAllEffectiveOwners', 'TestB124TenantRolePermissionsAreTenantScopedAndImmediate', 'TestB124OwnerRoleProtectsRequiredPermissionsAndLastAssignment', 'TestB125TenantCreateBootstrapsOwnerInOneRootUoW', 'TestB125ChildFailureRollsBackTenantAndMemberAndAllowsIdempotentRetry', 'TestB122TenantLifecycleUsesRootMySQLUnitOfWork', 'TestB122SuspendedTenantCredentialFailsAuthentication', 'TestB122TenantLifecycleRESTAndGRPCUseUnifiedExecutor', 'TestC98RealCrossApplicationTransferSharesOneExecutionScope', 'TestC9LocalCompositionUsesOneExecutorAndOneUoW', 'TestC9RemoteSagaStagesBusinessWriteAndOutboxAtomically', 'TestRESTAndGRPCShareOperationSecurityRuntime', 'TestCrossRoleLegacyScopeCannotEscalateGrant']
summary['mysql'] = verify_json('mysql.jsonl', mysql)
repeat = ['TestB126RoleRevokeAndMemberSuspendCannotRemoveAllEffectiveOwners', 'TestAG02OwnerInvariantUsesCurrentReadAfterSnapshot', 'TestAG02OwnerInvariantUsesCurrentReadAfterSnapshot/revoke', 'TestAG02OwnerInvariantUsesCurrentReadAfterSnapshot/suspend']
run(['go', 'test', '-json', '-count=20', '-timeout=5m', '-tags=integration', './integration', '-run', '^Test(B126RoleRevokeAndMemberSuspendCannotRemoveAllEffectiveOwners|AG02OwnerInvariantUsesCurrentReadAfterSnapshot)$'], 'owner-repeat.jsonl')
summary['repeat'] = verify_json('owner-repeat.jsonl', repeat, 20)

# Reuse actual B12.7 step bodies, not a weaker replacement runtime test.
lines = (biz / '.github/workflows/b12-7-runtime-qualification.yml').read_text().splitlines()
for name in ['Build Biz and exact Yunka CLI', 'Prove zero-argument Access + DeviceOps runtime closure']:
    positions = [i for i, line in enumerate(lines) if line.strip() == '- name: ' + name]
    assert len(positions) == 1
    i = next(i for i in range(positions[0] + 1, len(lines)) if lines[i].strip() == 'run: |') + 1
    body = []
    while i < len(lines) and (not lines[i].strip() or lines[i].startswith('          ')):
        body.append(lines[i][10:] if lines[i].strip() else '')
        i += 1
    script = evidence / ('runtime-' + str(positions[0]) + '.sh')
    script.write_text('\n'.join(body) + '\n')
    run(['bash', str(script)], script.stem + '.log', root / 'consumer')
assert not git('status', '--porcelain')
assert git('ls-remote', 'origin', 'refs/heads/main').split()[0] == head
api('repos/hvritual/biz/git/ref/heads/main', 'final-main-ref.json')
run(['git', 'format-patch', '--stdout', '--no-signature', '7d5afbe9cb4b849be462d9a7aed65877ed227700..HEAD'], 'consumer.patch')
run(['git', 'bundle', 'create', str(evidence / 'consumer.bundle'), 'HEAD', '^7d5afbe9cb4b849be462d9a7aed65877ed227700'], 'bundle-create.log')
run(['git', 'bundle', 'verify', str(evidence / 'consumer.bundle')], 'bundle-verify.log')
receipt = {'schemaVersion': 1, 'head': head, 'tree': tree, 'run': os.environ['GITHUB_RUN_ID'], 'mainReadback': True, 'postIntegrationVerified': True, 'signature': None, 'runtimePin': '6ba99c1440dc6c9416f6afd08f3282e35fa5a3fb', 'testPassEvents': summary, 'note': 'Unsigned execution evidence; not human approval or authority to change other refs.'}
(evidence / 'MAIN-RECEIPT.json').write_text(json.dumps(receipt, indent=2) + '\n')
print('AG02 actual main verified', head, tree, flush=True)
