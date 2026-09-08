from pathlib import Path
import os,runpy,subprocess,json,urllib.request
script=Path(__file__).with_name('docs.py')
sha=os.environ['BIZ_SHA'];pr=os.environ['BIZ_PR']
headers={'Accept':'application/vnd.github+json','Authorization':'Bearer '+os.environ['GH_TOKEN'],'X-GitHub-Api-Version':'2022-11-28'}
def get(path):
 with urllib.request.urlopen(urllib.request.Request('https://api.github.com/repos/hvritual/biz/'+path,headers=headers),timeout=30) as r:return json.load(r)
info=get('pulls/'+pr)
assert info['merged'] and info['head']['sha']==sha and info['merge_commit_sha']==sha, 'consumer integration not established'
assert get('git/ref/heads/main')['object']['sha']==sha, 'consumer main changed; re-evaluate qualification'
runpy.run_path(str(script),run_name='__main__')
p=Path('docs/STATUS.md');s=p.read_text();assert '| AG-02 | Real-consumer pilot qualified |' in s
s=s.replace('| AG-02 | Real-consumer pilot qualified |','| AG-02 | Complete / consumer-qualified / merged |',1)
s=s.replace('AG-03 is the next dependent pilot after actual Biz integration is verified.',f'Biz PR #{pr} is merged with merge SHA equal to the qualified head; its actual main ref was re-read. AG-03 is the next independent pilot.',1);p.write_text(s)
p=Path('docs/waves/AG-02-biz-encapsulation.md');s=p.read_text()
s+='''
## Qualification workspace disposition

Run `34189994020` passed all consumer behavior gates but failed on the separate
current-CLI workspace checksum write; it was not published. Run `34190521764`
passed the full suite and ten repeated owner regressions but failed a control
workspace-projection assertion. These are preserved control failures, not
successful end-to-end deliveries or additional consumer defects.

The final probe derives a private Go workspace from the exact canonical go.work
blob, preserving all five module source paths and its toolchain directive. Build
checksum metadata is recorded outside product source; it does not switch to a
remote old module, modify a product lock or suppress the three clean-worktree
checks. The Biz generator/runtime still uses its unchanged original source pin.
This CLI placement probe is not presented as full current-framework runtime
qualification. The framework documentation candidate runs its own normal gates.

The existing concurrent owner runtime test and both deterministic RR snapshot
orders are each repeated ten times; all required named pass events and absence
of failures/skips are checked in the final qualification artifact. Package pass
records are not counted as independent business tests.
'''
s+=f'\nActual consumer main integration was verified through PR #{pr} at `{sha}`, with the same qualified tree; no synthetic merge SHA is substituted.\n'
p.write_text(s)
subprocess.run(['git','add','docs/STATUS.md','docs/waves/AG-02-biz-encapsulation.md'],check=True)
subprocess.run(['git','diff','--cached','--check'],check=True)
