#!/usr/bin/env python3
from pathlib import Path
import importlib.util
import json

baseline_path = Path("tools/rpc-consumer-abi.json")
baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
consumers = baseline.get("consumers", [])
if baseline.get("schemaVersion") != 1 or len(consumers) != 1:
    raise SystemExit("C8.3.3: unexpected RPC consumer ABI baseline shape")
consumer = consumers[0]
if consumer.get("path") != "gateway/dispatcher/intercept/role/role_intercept.go" or consumer.get("receiver") != "RoleIntercept":
    raise SystemExit("C8.3.3: unexpected RPC consumer baseline target")

expected_old = {
    "BatchAddRuntimeApi": "9de88417b82aed40a8183886695e07169cb379da880ab7c2393e5676197571ca",
    "DeleteRuntimeApi": "7ebf99f71eb98843c02a05c20b106f28fb298831a70fb21361d934550f92859e",
    "OperateRoleAPI": "3caed6befd08683dc7f3b0d7eeaa24963f80b2384be0b8083c39170988541262",
}
if consumer.get("methods") != expected_old:
    raise SystemExit("C8.3.3: RPC consumer baseline moved before migration")

spec = importlib.util.spec_from_file_location("rpc_consumer_abi", "tools/check_rpc_consumer_abi.py")
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
source = Path(consumer["path"]).read_text(encoding="utf-8")
actual = {name: mod.digest_method(source, consumer["receiver"], name) for name in expected_old}
changed = {name for name in expected_old if actual[name] != expected_old[name]}
intended = {"BatchAddRuntimeApi", "OperateRoleAPI"}
if changed != intended:
    raise SystemExit(f"C8.3.3: unexpected business-method drift: {sorted(changed)}")
if actual["DeleteRuntimeApi"] != expected_old["DeleteRuntimeApi"]:
    raise SystemExit("C8.3.3: DeleteRuntimeApi method body changed unexpectedly")

consumer["methods"] = {name: actual[name] for name in expected_old}
baseline_path.write_text(json.dumps(baseline, indent=2) + "\n", encoding="utf-8")
print("C8.3.3: accepted intentional consumer behavior drift", actual)
