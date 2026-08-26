#!/usr/bin/env bash
set -euo pipefail

: "${RUNNER_TEMP:?}"
: "${GITHUB_WORKSPACE:?}"
: "${GITHUB_SHA:?}"

export_root="$GITHUB_WORKSPACE/tools/biz-domain-export"
rm -rf "$export_root"
mkdir -p "$export_root/internal/iam/infrastructure/persistence"
mkdir -p "$export_root/internal/site/infrastructure/persistence"
mkdir -p "$export_root/internal/device/infrastructure/persistence"

cat > "$export_root/go.mod" <<'EOF'
module github.com/hvritual/biz

go 1.25.0

toolchain go1.25.13

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	gorm.io/gorm v1.25.5
	yunka.io/framework v0.0.0
)

replace yunka.io/framework => ../../framework
replace yunka.io/pkg => ../../pkg
replace github.com/go-kit/kit v0.10.0 => ../../compat/go-kit-kit-log
EOF

cat > "$export_root/internal/iam/infrastructure/persistence/tenant.go" <<'EOF'
package persistence

type TenantPO struct {
	Name string `gorm:"column:name;type:varchar(200);not null"`
	Status string `gorm:"column:status;type:varchar(32);not null;index"`
}
EOF
cat > "$export_root/internal/iam/infrastructure/persistence/user.go" <<'EOF'
package persistence

type UserPO struct {
	Email string `gorm:"column:email;type:varchar(320);not null;uniqueIndex"`
	Status string `gorm:"column:status;type:varchar(32);not null;index"`
}
EOF
cat > "$export_root/internal/iam/infrastructure/persistence/membership.go" <<'EOF'
package persistence

type MembershipPO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	Status string `gorm:"column:status;type:varchar(32);not null;index"`
}
EOF
cat > "$export_root/internal/iam/infrastructure/persistence/role.go" <<'EOF'
package persistence

type RolePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	Name string `gorm:"column:name;type:varchar(100);not null"`
	Status string `gorm:"column:status;type:varchar(32);not null"`
}
EOF
cat > "$export_root/internal/iam/infrastructure/persistence/member_role.go" <<'EOF'
package persistence

type MemberRolePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	RoleID string `gorm:"column:role_id;type:varchar(160);not null;index" yunka:"-"`
}
EOF
cat > "$export_root/internal/iam/infrastructure/persistence/role_permission.go" <<'EOF'
package persistence

type RolePermissionPO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	RoleID string `gorm:"column:role_id;type:varchar(160);not null;index" yunka:"-"`
	Permission string `gorm:"column:permission;type:varchar(120);not null;index"`
	DataScope string `gorm:"column:data_scope;type:varchar(16);not null"`
}
EOF
cat > "$export_root/internal/iam/infrastructure/persistence/member_site.go" <<'EOF'
package persistence

type MemberSitePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	SiteID string `gorm:"column:site_id;type:varchar(64);not null;index" yunka:"-"`
}
EOF
cat > "$export_root/internal/iam/infrastructure/persistence/api_token.go" <<'EOF'
package persistence

import "time"

type APITokenPO struct {
	TokenHash string `gorm:"column:token_hash;type:varchar(64);not null;uniqueIndex" yunka:"-"`
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	ExpiresAt *time.Time `gorm:"column:expires_at" yunka:"-"`
	Disabled bool `gorm:"column:disabled;not null;default:false" yunka:"-"`
}
EOF
cat > "$export_root/internal/site/infrastructure/persistence/site.go" <<'EOF'
package persistence

type SitePO struct {
	Name string `gorm:"column:name;type:varchar(200);not null"`
}
EOF
cat > "$export_root/internal/device/infrastructure/persistence/device.go" <<'EOF'
package persistence

type DevicePO struct {
	SiteID string `gorm:"column:site_id;type:varchar(64);not null;index"`
	Name string `gorm:"column:name;type:varchar(200);not null"`
	Serial string `gorm:"column:serial;type:varchar(128);not null;index"`
	CreatedBy string `gorm:"column:created_by;type:varchar(64);not null;index"`
}
EOF

gofmt -w "$export_root/internal"
(cd "$GITHUB_WORKSPACE/app" && go build -o "$RUNNER_TEMP/yunka" ./cmd)
"$RUNNER_TEMP/yunka" init --root "$export_root" --db-prefix biz
"$RUNNER_TEMP/yunka" domain new --name iam --root "$export_root/internal" --global --no-rest --no-rpc
"$RUNNER_TEMP/yunka" domain new --name site --root "$export_root/internal" --no-rest --no-rpc
YUNKA_DOMAIN_TOOL_DIR="$GITHUB_WORKSPACE/.yunka/bin" "$RUNNER_TEMP/yunka" domain new --name device --root "$export_root/internal"
"$RUNNER_TEMP/yunka" domain generate --path "$export_root/internal/iam"
"$RUNNER_TEMP/yunka" domain generate --path "$export_root/internal/site"
YUNKA_DOMAIN_TOOL_DIR="$GITHUB_WORKSPACE/.yunka/bin" "$RUNNER_TEMP/yunka" domain generate --path "$export_root/internal/device"
YUNKA_DOMAIN_TOOL_DIR="$GITHUB_WORKSPACE/.yunka/bin" "$RUNNER_TEMP/yunka" domain check --root "$export_root/internal"
(cd "$export_root" && GOWORK=off go mod tidy && GOWORK=off go test ./...)

EXPORT_ROOT="$export_root" YUNKA_SHA="$GITHUB_SHA" python3 - <<'PY'
from pathlib import Path
import hashlib, json, os
root = Path(os.environ["EXPORT_ROOT"])
out = {"version": 1, "yunkaCommit": os.environ["YUNKA_SHA"], "files": []}
selected = [root / ".yunka/project.json"] + sorted((root / "internal").rglob("*"))
for path in selected:
    if not path.is_file():
        continue
    raw = path.read_bytes()
    out["files"].append({
        "path": path.relative_to(root).as_posix(),
        "sha256": hashlib.sha256(raw).hexdigest(),
        "content": raw.decode("utf-8"),
    })
bundle = root.parent / "biz-domain-export.bundle.json"
bundle.write_text(json.dumps(out, indent=2, ensure_ascii=False) + "\n")
PY
