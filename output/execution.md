# Execution Record: gb-537

## Scope

- Project: `gb-537`
- Product name: `CertRollover - PKI Certificate Rollover Impact`
- Validation date: 2026-08-22 (Asia/Tokyo)
- Actual ports: frontend `18537`, backend `19537`, PostgreSQL `57537`, runtime smoke `20537`
- Measured Go functional scale: 3,761 lines across 42 functional Go files

## Build and test evidence

| Command | Actual result |
| --- | --- |
| `go work sync` | Passed |
| `go build ./backend/...` | Passed |
| `go vet ./backend/...` | Passed |
| `go test ./backend/...` | Passed |
| `go test -race ./backend/...` | Passed |
| `cd backend && go build ./...` | Passed |
| `cd backend && go vet ./...` | Passed |
| `cd backend && go test ./...` | Passed |
| `npm --prefix frontend ci` | Passed after removing an interrupted local dependency directory and reinstalling from the lockfile |
| `npm --prefix frontend run build` | Passed (TypeScript and Vite build) |
| `npm --prefix frontend test` | Passed: 5 files, 9 tests |
| `python3 /Users/gaobo/.codex/skills/go-annotation-pipeline/scripts/project_scale.py .` | Passed: 3,761 lines, 42 files |
| `python3 /Users/gaobo/.codex/skills/go-annotation-pipeline/scripts/runtime_smoke.py .` | Passed: `HTTP 200` from `http://127.0.0.1:20537/healthz` |

## Compose evidence

`docker compose config --quiet` completed successfully. `docker compose up -d --build` produced three healthy services:

| Service | Actual status | Published port |
| --- | --- | --- |
| `pki-certificate-rollover-impact-db` | healthy | `57537:5432` |
| `pki-certificate-rollover-impact-backend` | healthy | `19537:8080` |
| `pki-certificate-rollover-impact-frontend` | healthy | `18537:80` |

Health assertions succeeded:

| Method and path | Expected | Actual |
| --- | --- | --- |
| `GET http://127.0.0.1:19537/healthz` | `200` | `200` |
| `GET http://127.0.0.1:18537/api/healthz` | `200` | `200` |
| `GET http://127.0.0.1:18537/` | `200` | `200` |

## API smoke evidence

Authentication succeeded with local `admin`, `operator`, and `reviewer` accounts. The following results were observed against the healthy Compose stack.

| Method and path | Expected | Actual assertion |
| --- | --- | --- |
| `POST /api/v1/auth/login` | `200` | `200` for `admin`, `operator`, and `reviewer` |
| `GET /api/v1/trust-anchors` | `200` | `200` |
| `GET /api/v1/certificate-chains` | `200` | `200` |
| `GET /api/v1/dependent-services` | `200` | `200` |
| `GET /api/v1/rollover-scenarios` | `200` | `200` |
| `GET /api/v1/audit-logs` | `200` | `200` |
| `GET /api/v1/trust-anchors` without token | `401` | `401` |
| `POST /api/v1/dependent-services` as operator | `201` | `201`; created `AUDIT-SMOKE-537` with id `4` |
| `POST /api/v1/rollover-scenarios` as operator | `201` | `201`; created `Audit smoke offline rollover` with id `2` |
| `POST /api/v1/rollover-scenarios/2/simulate` with `Idempotency-Key: audit-smoke-537-simulation` | `201` | `201`; state became `simulated` |
| `POST /api/v1/rollover-scenarios/2/transition` to `verified` by creator | `409` | Initial Compose run exposed an incorrect `403`; after the source repair, isolated SQLite HTTP verification returned `409 REVIEWER_SEPARATION_REQUIRED`. Docker image rebuild remained blocked by disk exhaustion. |
| `POST /api/v1/rollover-scenarios/2/transition` to `ready` | `200` | `200`; state `ready` |
| `POST /api/v1/dependent-services` as reviewer | `403` | `403 FORBIDDEN` |
| `GET /api/v1/rollover-scenarios/2` | `200` | `200`; state `ready`, path evidence length `6` |

Each core entity therefore traversed a real backend route: trust-anchor, certificate-chain, dependent-service, and rollover-scenario. Both success and permission/error paths were exercised.

## Repairs made during audit

- Corrected Vitest setup to use `jsdom` with the project setup file.
- Fixed a TypeScript matcher generic unsupported by the installed Vitest typings.
- Corrected readonly fixture typing in `DependencyGraph` tests.
- Repaired verification error precedence: a scenario creator without reviewer permission now receives the intended `409 REVIEWER_SEPARATION_REQUIRED`; unrelated unauthorized verification attempts remain `403`. The repaired HTTP path is verified in an isolated local SQLite runtime below.
- Added service-level regression coverage for self-verification separation.
- Added this README with deployment, API, state-machine, safety-boundary, and operations documentation.

## Local post-repair HTTP verification

The current source was started on `http://127.0.0.1:21537` with an isolated SQLite database (`/tmp/gb537-postrepair.sqlite`). This verification was used because Docker's build layer ran out of disk space while rebuilding the patched backend image.

| Action | Actual result |
| --- | --- |
| `GET /healthz` | `200` |
| Login as `operator` | `200` |
| Create `Post-repair separation check` | `201`, scenario id `2` |
| Simulate with a new idempotency key | `201` |
| Transition to `ready` | `200` |
| Transition to `executing` | `200` |
| Creator transitions to `verified` | `409 REVIEWER_SEPARATION_REQUIRED` |

## Browser acceptance

仅使用 Codex 内置 Browser（IAB），没有使用外部浏览器。以 `operator / operator123` 登录后，真实检查了 `/anchors`、`/chains`、`/dependencies` 和 `/rollovers`：根证书、证书链离线验证、服务依赖图和冻结轮换推演均由 `/api/v1` 数据渲染。轮换页面显示已冻结输入哈希、推演时间点、两条断裂路径和受影响服务，符合离线推演边界。

在 390 x 844 视口重新打开 `/rollovers`，取得 `clientWidth=390`、`scrollWidth=390`、`bodyWidth=390`，未出现横向溢出或遮挡。Browser console 日志为空，页面网络/交互没有阻断错误。

- 桌面截图：`output/browser-desktop.png`
- 移动端截图：`output/browser-mobile-390.png`

## Cleanup

`docker compose down -v --remove-orphans` completed successfully after both initial API validation and the subsequent Browser acceptance. It removed the frontend, backend, and database containers, the `pki-certificate-rollover-impact_default` network, and the `pki-certificate-rollover-impact-postgres-data` named volume. Subsequent checks found no matching containers, networks, or named volumes. The isolated SQLite runtime was also stopped; its temporary database file was removed. The runtime smoke process exits after its health assertion.

## Commit references

Implementation commit: `4f327736eea53eac5fafd18fbaa41cd3700f074f`
