from pathlib import Path

root = Path(__file__).resolve().parents[1]
path = root / "pkg/contract/artifact_test.go"
text = path.read_text()
text = text.replace('import (\n\t"os"', 'import (\n\t"fmt"\n\t"os"', 1)
text = text.replace('\t"testing"\n)', '\t"testing"\n\n\t"yunka.io/pkg/operationplan"\n)', 1)
old = 'if string(first.OperationPlans) != "{\\n  \\"schemaVersion\\": 1,\\n  \\"operations\\": []\\n}\\n" {\n\t\tt.Fatalf("unexpected operation plans: %s", first.OperationPlans)\n\t}'
new = 'wantOperationPlans := fmt.Sprintf("{\\n  \\"schemaVersion\\": %d,\\n  \\"operations\\": []\\n}\\n", operationplan.SchemaVersion)\n\tif string(first.OperationPlans) != wantOperationPlans {\n\t\tt.Fatalf("unexpected operation plans: %s", first.OperationPlans)\n\t}'
if old not in text:
    raise SystemExit("artifact schema assertion fragment not found")
path.write_text(text.replace(old, new, 1))
