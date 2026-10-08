import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import zlib

control = os.environ['CONTROL_SHA']
def payload(name, repairs):
    source = subprocess.check_output(['git','show',control+':.control/'+name])
    envelope = json.loads(source)
    encoded = envelope['data']
    for old, new in repairs:
        encoded = encoded.replace(old,new)
    raw = zlib.decompress(base64.b64decode(encoded,validate=True))
    assert hashlib.sha256(raw).hexdigest() == envelope['sha256'], name+' integrity mismatch'
    return raw

runtime = payload('http-binding-runtime.json', [('CfraPPq5w','CfraPq5w'),('PiX3ULyWQ','PiX3yWQ')])
implementation = payload('http-binding-implementation.json', [('B5oT5oTq9','B5oTq9'),('0Yvjuu0YvHY','0YvHY')])
if sys.argv[1] == 'runtime':
    files = json.loads(runtime)
    assert set(files) == {'pkg/contract/http_binding_runtime_test.go'}
    for name, content in files.items():
        path = Path(name)
        assert not path.exists()
        old = 'authz.RequireAuthorizedOperation(ctx,id)'
        assert old in content
        content = content.replace(old,'authz.RequireAuthorizedOperation(ctx,authz.OperationID(id))')
        path.write_text(content)
        print(name, hashlib.sha256(content.encode()).hexdigest())
elif sys.argv[1] == 'implementation':
    path = Path(os.environ['RUNNER_TEMP'])/'http-binding.patch'
    # difflib does not emit Git's new-file mode header; preserve all hunk bytes.
    text = implementation.decode().replace('\n--- /dev/null\n','\nnew file mode 100644\n--- /dev/null\n')
    path.write_text(text)
    subprocess.run(['git','apply','--check',str(path)],check=True)
    subprocess.run(['git','apply',str(path)],check=True)
else:
    raise SystemExit('unknown installation stage')
