# Application content evidence matrix

状态：`planned` 未执行；`delegated-unrun` 已交接但无运行证据；`passed/failed` 仅在绑定 candidate、命令和 artifact 后填写。

| ID | AC | Required proof | Runner | Status |
| --- | --- | --- | --- | --- |
| E-C01 | AC-C1 | 26 callable + 2 integrations + 4 providers + 4 dormant 静态核对 | spec | planned |
| E-C02 | AC-C2 | application/content dependency scan | local Go | planned |
| E-C03 | AC-C3 | separators/dot/absolute/drive/UNC/device/reserved lexical table | unit | planned |
| E-C04 | AC-C3 | symlink/junction/reparse/deepest parent/root swap；paired quarantine same-filesystem/non-public/no-overlap | Windows+Linux fs | planned |
| E-C05 | AC-C4 | same/different digest、concurrent、fixed-pending regression | fs integration | planned |
| E-C06 | AC-C4 | write/sync/close/no-replace/dir-sync/unsupported fs failpoints | fake+fs | planned |
| E-C07 | AC-C4/10 | file + owner row + projection four-state Verify | Mongo+fs | planned |
| E-C08 | AC-C5 | multipart presence/one-file/stream limit/invalid config | app/controller | planned |
| E-C09 | AC-C5 | DecodeConfig-before-decode、width/height/pixel/allocation/frame budgets、valid/invalid GIF/JPEG/PNG/BMP corpus and cleanup | decoder | planned |
| E-C10 | AC-C6 | image owner/blog/share/foreign/FromFileId/spoof ACL | service/repo | planned |
| E-C11 | AC-C6 | attachment owner/update/read/public/share/foreign ACL | service/repo | planned |
| E-C12 | AC-C6 | invalid ID/open/stat/DB failures, no panic/empty success | negative | planned |
| E-C13 | AC-C7 | album CRUD/default/name/count/cross-owner | Mongo | planned |
| E-C14 | AC-C8 | tar.gz content/entry sanitize/duplicate names | archive unit | planned |
| E-C15 | AC-C8 | write/close failures、concurrency、temp cleanup | archive failpoint | planned |
| E-C16 | AC-C9 | public-only IP/DNS rebinding/redirect/scheme/userinfo；无 allowlist/fallback | remote fake | planned |
| E-C17 | AC-C9 | timeout/redirect-size cap/non-2xx/invalid image/cleanup | remote integration | planned |
| E-C18 | AC-C10 | Files missing/empty/nonempty Reconcile | notes-content | planned |
| E-C19 | AC-C10 | Copy frozen manifest/stable destination/no new-source retry | notes-content | planned |
| E-C20 | AC-C10 | Delete post-commit repair，不重复/回滚 USN/history；durable quarantine manifest 不进入 notes receipt | notes-content | planned |
| E-C21 | AC-C10 | unknown Apply/Verify，无第二 receipt state | failpoint | planned |
| E-C22 | AC-C11 | self-contained serializer、恶意 HTML stripping、direct argv/metachar/timeout/cancel/executable/input/output cleanup | fake exec | planned |
| E-C23 | AC-C11/14 | local-file deny、zero outbound、file/private/metadata/redirect/CSS/script-fetch negative cases、secret absence in URL/argv/env/log/error/artifact | sandbox/container | delegated-unrun |
| E-C24 | AC-C11 | legacy appKey bind-and-discard、PDF magic/readability/size/name/Content-Type mapper | focused+HTTP | planned |
| E-C25 | AC-C12 | per-action method/presence/status/body/binary Golden | real HTTP | delegated-unrun |
| E-C26 | AC-C12 | album iframe/paste/upload/download visible behavior | real browser | delegated-unrun |
| E-C27 | AC-C13 | Mongo/file multi-write + durable manifest version/CAS/torn-write + legacy no-OperationID restart + terminal retention/GC recovery | Mongo 8 local fs | planned |
| E-C28 | AC-C13/14 | kill/restart + cross-host/persistent-volume failpoint | delivery | delegated-unrun |
| E-C29 | AC-C14 | Linux/container real wkhtmltopdf success/failure/timeout artifact | delivery | delegated-unrun |
| E-C30 | AC-C14 | non-root writable roots、volume restart、cleanup | delivery | delegated-unrun |
| E-C31 | all | gofmt、focused/full test、scan、vet、build、JS if touched、diff check | local/CI | planned |

## Integrity rules

1. planned、delegated-unrun 都不是 passed。
2. fake executable 不关闭 E-C29；installed package/build 不替代 runtime artifact。
3. controller direct call/mock 不关闭 E-C25/E-C26；same-host fs 不关闭 E-C28。
4. passed 必须记录 commit/tree、run/attempt、time、environment、command、exit code、artifact/digest 和脱敏 failure/skip。
