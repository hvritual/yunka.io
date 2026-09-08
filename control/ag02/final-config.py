"""Pin staged delivery recipes to the independently reviewed final consumer head."""
from pathlib import Path
import sys

here = Path(__file__).resolve().parent
head = '3519e7ee6e51e33984669871e4f32a55a3597d9f'
tree = '3f8926b459fb78688fe0f39e9c4acfb6d93b3b47'
old = 'df6cccf32fc2557f6a22a26b4a966a58b775ff27'
oldtree = 'a320d20874d4054ce8a0de18482aa984274548bf'


def once(text, before, after):
    assert text.count(before) == 1, before
    return text.replace(before, after)


if sys.argv[1:] == ['consumer']:
    source = (here / 'post-main.py').read_text()
    source = once(source, "head = '" + old + "'", "head = '" + head + "'")
    source = once(source, "tree = '" + oldtree + "'", "tree = '" + tree + "'")
    source = once(source, "'`df6cccf`' in review['body']", "'`3519e7e`' in review['body']")
    source = once(source, "'df6cccf' in x['body']", "'3519e7e' in x['body']")
    before = 'run_ids = [34187433447, 34187433459, 34187433499, 34187433520, 34187433457, 34187433495, 34187433456, 34187433539, 34187433458]'
    after = '''runs = api('repos/hvritual/biz/actions/runs?head_sha=' + head + '&per_page=100', 'all-final-pr-runs.json')['workflow_runs']
wanted = ['c9-biz-pressure', 'b12-8-framework-pressure-disposition', 'delivery-workspace-isolation', 'b12-7-runtime-qualification', 'b12-6-concurrency-mysql', 'b12-5-tenant-bootstrap-mysql', 'b12-multitenant-access-pressure', 'b12-4-tenant-role-mysql', 'b12-3-tenant-member-mysql']
run_ids = []
for name in wanted:
    matches = [r for r in runs if r['name'] == name and r['event'] == 'pull_request']
    assert matches, name
    latest = max(matches, key=lambda r: r['id'])
    assert latest['conclusion'] == 'success' and latest['status'] == 'completed', name
    run_ids.append(latest['id'])'''
    source = once(source, before, after)
    exec(compile(source, str(here / 'post-main.py'), 'exec'), {'__name__': '__main__'})
elif sys.argv[1:] == ['docs']:
    source = (here / 'reconcile-final.py').read_text()
    source = once(source, "assert receipt['head'] == '" + old + "'", "assert receipt['head'] == '" + head + "'")
    source = once(source, "assert receipt['tree'] == '" + oldtree + "'", "assert receipt['tree'] == '" + tree + "'")
    exec(compile(source, str(here / 'reconcile-final.py'), 'exec'), {'__name__': '__main__'})
    p = Path.cwd() / 'docs/waves/AG-02-tenant-encapsulation.md'
    s = p.read_text()
    s = once(s, 'integrates `' + old + '`, tree `' + oldtree + '`', 'integrates `' + head + '`, tree `' + tree + '`')
    s = once(s, 'Three separate normal local-Git commits implement', 'The first three separate normal local-Git commits implement')
    s = once(s, 'Nine fresh existing PR workflows passed the final source,', 'Nine existing PR workflows passed the df6cccf source,')
    marker = 'The [post-integration consumer verification and framework-documentation qualification run '
    assert s.count(marker) == 1
    addition = '''Independent review of df6cccf found two recurring-coverage omissions: the new snapshot test was not selected by ordinary integration workflows, and the readiness script was absent from workflow path filters. The fourth local-Git commit `3519e7ee6e51e33984669871e4f32a55a3597d9f` adds the exact snapshot test to B12.6, includes its source in both event filters, and includes the readiness script in both B12.7 filters. Existing checks are retained. [Qualification run 34188061220](https://github.com/hvritual/biz/actions/runs/34188061220) replays the complete consumer, repeated owner and actual runtime gates on this exact final source; its receipt explicitly separates qualification from Runner workflow-permission publication. Final exact-head review and all nine fresh normal PR checks are independently re-read by the post-integration run, not inherited from df6cccf.\n\n'''
    s = s.replace(marker, addition + marker)
    p.write_text(s)
else:
    raise SystemExit('expected consumer or docs')
