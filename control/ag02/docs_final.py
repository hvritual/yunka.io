from pathlib import Path
import subprocess,hashlib,urllib.request,os,json,time
# A bounded dependency wait never converts pending/failure into acceptance.
headers={'Accept':'application/vnd.github+json','Authorization':'Bearer '+os.environ['GH_TOKEN']}
for attempt in range(37):
 req=urllib.request.Request('https://api.github.com/repos/hvritual/biz/actions/runs/34192294589',headers=headers)
 with urllib.request.urlopen(req,timeout=30) as response: result=json.load(response)
 if result['status']=='completed':
  assert result['conclusion']=='success', 'actual consumer main acceptance failed'
  break
 if attempt==36: raise RuntimeError('INCOMPLETE: consumer main acceptance not complete')
 time.sleep(5)
p=Path(__file__).with_name('docs_source.py');data=p.read_bytes()
assert hashlib.sha1(b'blob '+str(len(data)).encode()+b'\0'+data).hexdigest()=='c11be8467ca2355a7fc475c119308d942aef017e'
source=data.decode();assert '34191937549' in source
source=source.replace('34191937549','34192294589')
exec(compile(source,str(p),'exec'),{'__name__':'__main__','__file__':str(p)})
p=Path('docs/waves/AG-02-biz-encapsulation.md')
p.write_text(p.read_text()+'''
## Actual-main acceptance correction

The first post-integration control run `34191937549` completed the consumer,
MySQL, repeated-owner and full runtime gates, but its targeted package selector
omitted `internal/architecture`; its required six-test inventory correctly failed.
It is not marked successful. Run `34192294589` adds that existing package without
lowering the inventory and repeats all gates; the six required test names are
explicitly matched. Product source and both qualified identities are unchanged.
''')
subprocess.run(['git','add',str(p)],check=True)
subprocess.run(['git','diff','--cached','--check'],check=True)
