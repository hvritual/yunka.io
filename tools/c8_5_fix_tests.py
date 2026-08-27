from pathlib import Path
p = Path('pkg/contract/application_codegen_test.go')
s = p.read_text()
old = '''\trest := byPath["device/transport/rest/zz_yunka_device_management_rest_adapter_gen.go"]\n\tif !strings.Contains(rest, `handler.authorize(request, "/device.v1.DeviceApplication/GetMachine")`) || !strings.Contains(rest, "handler.application.GetMachine(request.Context(), wire)") || !strings.Contains(rest, "handler.authorizer.Authorize") {\n\t\tt.Fatalf("REST does not reuse policy/authorizer/application port:\\n%s", rest)\n\t}\n'''
new = '''\trest := byPath["device/transport/rest/zz_yunka_device_management_rest_adapter_gen.go"]\n\tif !strings.Contains(rest, `handler.runtime.Prepare(request.Context(), "/device.v1.DeviceApplication/GetMachine", wire)`) || !strings.Contains(rest, "handler.application.GetMachine(secured, wire)") || !strings.Contains(rest, "runtime     authz.OperationRuntime") {\n\t\tt.Fatalf("REST does not use shared C8.5 operation runtime before application:\\n%s", rest)\n\t}\n\tif strings.Contains(rest, "handler.authorizer") || strings.Contains(rest, "ResolvePolicy") {\n\t\tt.Fatalf("REST retained transport-local authorization logic:\\n%s", rest)\n\t}\n'''
if old not in s:
    raise SystemExit('legacy REST assertion not found')
s = s.replace(old, new, 1)
old_policy = '''\tif !strings.Contains(policy, `OperationGetMachine authz.OperationID = "device.machine.get"`) || !strings.Contains(policy, `"device.machine.read"`) || !strings.Contains(policy, `"/device.v1.DeviceApplication/GetMachine"`) {\n'''
new_policy = '''\tif !strings.Contains(policy, `OperationGetMachine authz.OperationID = "device.machine.get"`) || !strings.Contains(policy, `"device.machine.read"`) || !strings.Contains(policy, `"/device.v1.DeviceApplication/GetMachine"`) || !strings.Contains(policy, "func Permissions() []authz.PermissionKey") {\n'''
if old_policy not in s:
    raise SystemExit('policy assertion not found')
s = s.replace(old_policy, new_policy, 1)
p.write_text(s)
