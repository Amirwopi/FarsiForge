# Active Context

Updated: 2026-10-09

## Current checkpoint

- The main repository is `D:\FarsiForge`; the format-tools repository is `D:\FarsiForgeTools`.
- The requested folder transfer to `D:\FarsiForge-dev` was canceled. Do not resume it. Partial destination artifacts were left untouched; the two source repositories remain the working copies.
- The current worktrees contain uncommitted changes. Preserve `D:\FarsiForge\Tools.rar` and `D:\FarsiForgeTools\internal\translation\`; do not commit or push unless directly requested.
- Synthetic Godot end-to-end check passed: inject an existing Persian `.translation` into a disposable PCK, rebuild it, build an FFP1 package, apply with the C# CLI, uninstall, and compare the restored PCK byte-for-byte with the original.
- Read-only real MOLDRISE v1.0.5 extraction succeeded: PCK v4 / Godot 4.7.0; 3,789 strings from 467 recovered text files after filtering editor add-on files and dictionary keys. All entries have unique IDs plus source file/line context. This build has no `.translation` catalog, so its recovered scene/script literals cannot currently be injected by the main Godot injector.
- Full main-repository Go test, vet, and build passed after parser, PCK version, game-name, and persistence changes. `graphify update .` completed.
- Two scratch directories from the first real extraction remain under `%TEMP%`. The prior C: full report is stale: latest check reports 12,474,654,720 bytes free on C: and 69,647,757,312 bytes free on D:. Keep large new scratch output on D:; do not bypass the earlier automatic-review rejection for recursive cleanup.
- Injection now records SHA-256 hashes of source files for successfully staged targets. Patch creation rejects stale staged output if a game file changed after injection. Full `go test ./... -count=1`, `go vet ./...`, `go build ./...`, and `git diff --check` pass after this change.

## Next work

- `subst` currently reports no Windows substituted drives. Volume GUID/mount-point alias identity is still unverified.
- Added and passed a deterministic Windows interrupted-save test: a no-delete-share handle blocks replacement; the prior project file remains byte-identical and the temporary file is cleaned up.
- The read-only Orc Massage Unity project contains 607/607 `raw_*` fallback entries, including false positives and real dialogue/UI. Those entries lack writable field identity; injection now rejects them up front and the UI labels the limitation. Unity write-back is still unverified. Current validation after this change passes Go tests/vet/build plus frontend lint and TypeScript check.
- Continue the open items in root `todo.md`, starting with the full diff audit, Windows volume-alias identity, deterministic project-save failure coverage, and evidence-based format validation.
- Validate Godot, Unity, Unreal, and other formats against representative real data only through read-only game inputs and disposable outputs. Synthetic results do not establish broad game compatibility.

## Safety boundaries

- Keep real game installations read-only; extraction/injection tests must stage under temporary or configured project storage.
- Do not resume the canceled directory copy or clean up its partial target artifacts without a new request.
- Do not send tasks to OpenCode.
