# FarsiForge handoff TODO

Last updated: 2026-10-09

This file intentionally lists only unfinished work. Remove each item as it is completed and record lasting architectural facts or measured results in `memory-bank/`.

## Scope and safety constraints

- Product goal: a dependable Persian localization workflow that extracts actual dialogue, UI, journals, notes, item names/descriptions, and other game text; supports format-correct injection; and builds reversible patches.
- Do not infer format support from engine detection. Prove extraction and write-back separately with real representative files.
- Real game installations are read-only. Use disposable copies or test workspaces under `%TEMP%` / configured project storage; never point test output at installed game directories.
- Preserve unrelated local data: `D:\FarsiForge\Tools.rar` and untracked `D:\FarsiForgeTools\internal\translation\`.
- Do not commit or push the main `Amirwopi/FarsiForge` repository unless explicitly requested.

## P0 — Finish reviewing the current patch/install changes

- [x] Continued the FFP1 v2 source review locally and fixed the confirmed copy-from-base reparse-point escape below.
- [x] Manual FFP1 audit found that copy-from-base records could read through a directory reparse point outside the game root. The patcher now checks each base path during preflight and immediately before opening it. A Windows Go-to-C# regression test creates a directory symlink and confirms the patch is rejected without leaving a target or `.ffnew` file.
- [x] During the persistence/UI save review, `SaveTranslations` now rejects missing/duplicate IDs, IDs absent from the current project, duplicate project IDs, and invalid statuses before saving. Regression tests confirm stale and duplicate submissions leave persisted translations unchanged.
- [x] Re-extraction translation matching now uses a comparable tuple of source identity fields instead of NUL-delimited concatenation, preventing field-boundary collisions from transferring a translation to a different entry.
- [ ] Re-read the full diff for project persistence, injectors, patch writer/installer, logger, CLI, UI, and tests. Check failures and partial outputs; ensure no unrequested generated files or credentials were added.
- [x] Verify Windows case-insensitive project-path identity, symlink alias normalization, and replacement of an existing project file. `go test ./pkg/core -count=1` passes.
- [ ] Investigate Windows volume aliases. `subst` reports no current substituted drives; volume GUID and mount-point aliases remain unverified.
- [ ] Review the two retained PCK scratch directories under `%TEMP%\FarsiForge-real-godot-20261009` and `%TEMP%\FarsiForge-moldrise-projects-bc49b7813757496ba65d396dd1eebf90` for safe cleanup. The earlier 0-byte C: reading is stale: latest check reports 12,474,654,720 bytes free on C: and 69,647,757,312 bytes free on D:. Do not bypass the earlier automatic-review rejection for recursive deletion; keep new large scratch output on D:.

## P1 — Validate end-to-end injection by actual format

- [x] Read-only real Godot extraction on MOLDRISE v1.0.5: the external standalone PCK is format v4 / Godot 4.7.0 and contains 4,620 files. GDRE recovered the project; the main pipeline extracted 3,789 strings from 467 text files. After inspection, it excludes 106 editor add-on source files and dictionary keys, yielding 0 editor-addon entries; 413 runtime/add-on entries remain. Long dialogue and main-menu accessibility/UI text were confirmed in authored resources. IDs are unique; every entry has a non-empty synthetic literal key, a container/file, context, and source line. This is read-only extraction evidence, not proof of complete semantic coverage.
- [ ] Audit remaining extracted candidates for item names/descriptions, notes/newsletters, and false positives. In this build, inspected item paths yielded mostly state/control labels and a `newspaper` name; note/newsletter image and texture content is not covered by ordinary source-string extraction. Do not claim full item/note coverage.
- [ ] Validate write-back and gameplay loading against a disposable copy of the Godot 4.7.0 game. The inspected PCK has no `.translation`/CSV catalog; current main injection only supports existing Persian `.translation` resources and cannot write back these recovered `.gd/.tscn/.tres` literals yet. Keep the installed game read-only.
- [ ] Review the current Godot 4.3+ / PCK v3 result (8,123 entries, 390 files: 7,014 `.tscn`, 1,024 `.gd`, 85 `.tres`) against the recovered files. Confirm dialogue/UI/items/notes are real player-facing strings, quantify false positives and omissions, and improve IDs/context so translators can identify where each line appears. The main extractor now maps `.translation` entries when the matching source CSV is present; validate the complete PCK recovery path and semantic coverage.
- [ ] Unity: use a disposable copy of a real Unity game. Confirm exact source identity matching, modified asset list, output paths, typetree/MonoBehaviour handling, and that the game loads the staged/repacked result. Record Unity version, game build, asset types, counts, and skipped/error counts. Do not claim all Mono/IL2CPP assets are supported from synthetic tests.
- [ ] Review the read-only Orc Massage extraction checkpoint (Unity 2021.2.3): 607 entries from 5 files (383 `.assets`, 224 extensionless levels), unique non-empty IDs. The external project is `%TEMP%\FarsiForge-validation-orcmassage-20261009\0eb3557443eb32c3d66b32d1da7daf8e\project.json`. Audit found all 607 paths are `raw_*` byte-scan candidates: this includes dialogue/UI text, controls, repeated placeholder text, Unity asset-store documentation, and UI Toolkit style declarations. These entries do not have verified typetree field identity; Unity injection now rejects translated raw candidates before running tools and the UI labels them as currently non-injectable. The install is about 6.21 GB; choose a bounded disposable asset fixture for write-back validation and do not copy or modify the installed game.
- [ ] Unreal `.locres`: validate real shipped versions and language/culture handling with `D:\FarsiForgeTools` export/import; ensure keys, namespace, source hashes, and unknown entries survive roundtrip. Separately establish whether `.pak` rebuild is implemented; do not imply it based on `.locres` extraction from a pak.
- [ ] Generic text: validate encoding/BOM, CRLF, duplicate keys, delimiter/escaping rules, and exact roundtrip on representative files; current path is only UTF-8 key/value and must fail clearly outside that contract.
- [ ] Confirm patch output includes every modified path and original-base hash, and rejects missing or ambiguous mappings rather than silently omitting a translation. Source hashes are now bound at injection and stale output is rejected; complete the remaining full diff audit and verify every injector's path/provenance handling.

## P2 — Complete the separate FarsiForgeTools repository review

- [x] Implement Godot 4 binary `OptimizedTranslation` read/write in `D:\FarsiForgeTools\internal\translation\`. Added two synthetic Godot 4.5.1 fixtures (single and multi-entry), corrupt/truncated-input checks, source-key hash resolution, and exact-key/ID-validated updates. It preserves untranslated entries and the original hash layout. Verified rewritten binaries load in Godot 4.5.1.
- [x] Add `fftools translation export/import`: export requires a companion Godot CSV and exact source-key hashes because the binary resource does not contain source strings. Import validates source key plus stable resource ID, keeps unmentioned entries, and emits a new file. On local FA/EN samples, all 269 entries mapped to the CSV key catalog; the EN resource exercised 136 Smaz-compressed strings. Godot 4.5.1 loaded both outputs, and `get_message()` returned the changed Persian value for its exact key.
- [x] Wire `D:\FarsiForgeTools` translation export into the main FarsiForge Godot extractor. A read-only local integration test mapped 4,842 entries from 18 locale resources (269 per locale) using the matching CSV; IDs include the resource path and hash ID, source language and locale are retained in context, and 4,826 existing non-empty values are marked translated.
- [x] Add `fftools pck rebuild` for standalone unencrypted PCK v2/v3/v4. It preserves unchanged entries, recomputes MD5 and offsets, rejects encrypted/sparse flags, traversal paths, symlink replacements, unknown files, and writes atomically. The main injector now updates exact-key/ID-matched entries in an existing Persian `.translation` resource and stages the rebuilt full PCK under its game-relative path.
- [x] Preserve Godot container identity in extracted project entries and translation merge keys so same-named resources from different PCKs do not collide. Synthetic PCK v4 injection passed through the Go injector and FFP staging; Godot 4.5.1 loaded the staged pack and returned the edited Persian value. The installed source PCK remained unchanged.
- [x] Exercise the full synthetic Godot install path: inject a changed `.translation`, rebuild the PCK, build an FFP1 package with the original PCK hash, apply it with the C# patcher, then uninstall it. The installed file matched the staged PCK and uninstall restored the source PCK byte-for-byte in a disposable game directory.
- [ ] Validate write-back against a real disposable Godot game PCK and runtime. Current engine validation is synthetic; the available local PCK reports Godot 4.7 and cannot be validated by the installed Godot 4.5.1 runtime. Embedded PCKs, encrypted/sparse bundles, scene/script text, and creating a missing Persian locale remain unsupported and must fail clearly.
- [ ] Validate `.locres` v0-v3 and Godot PCK v2-v4 against fixtures. Keep extraction/injection roundtrip coverage per version; reject unsupported variants explicitly.
- [ ] Expand Godot translation fixtures beyond the synthetic format-6 sample, validate additional engine/resource versions, and exercise translated strings in a disposable project/game. Context and plural variants are not represented in `OptimizedTranslation`; keep that limitation visible and find the originating catalog/resource for those cases.

## P3 — Close extraction coverage gaps with evidence

- [ ] RAGE/GTA V: implement or explicitly scope encrypted RPF7 traversal (NG key/table discovery, nested archives, decompression) and resolve GXT2 content from real files. Validate using copies of game files; document encryption/build constraints. Current loose GXT2 parsing is not GTA V archive support.
- [ ] FromSoftware: replace brittle guessed archive/language selection with format-driven discovery where supported; cover BND3 and additional DCX variants only with fixtures. Revalidate ELDEN RING Data0 counts and add a second game/build. Injection remains unsupported until format-correct write/repack exists.
- [ ] Godot: validate PCK v2/v3/v4 traversal independently and cover text embedded in scripts, scenes, and resources. The 8,123-entry PCK v3 result is a separate earlier sample; current MOLDRISE evidence is PCK v4 / engine 4.7.0. Neither count is a completeness guarantee.
- [ ] Audit each detected engine in `pkg/detection` against the extractor/injector registry. Mark detection-only engines plainly in UI/docs, and return an explicit unsupported error instead of generic text mutation.
- [ ] Build a fixture matrix for dialogue, subtitles, UI, journals/notes, item names/descriptions, tutorials, and quest text. Report counts by asset/type and identify cases where strings are embedded in scripts, fonts, textures, or encrypted containers.

## Previous pass verification baseline

- Current continuation: opt-in `TestGodotInjectorPCKIntegration` passed with the disposable Godot-generated v4 PCK and local `fftools` / C# patcher. It verified injection, FFP1 build, apply, uninstall, and byte-for-byte restoration. Later Godot extraction/parser changes, exact PCK engine-version detection, project game-name persistence, translation-save validation, and structured merge identity all passed `go test ./... -count=1`, `go vet ./...`, and `go build ./...`; `graphify update .` refreshed the graph. `git diff --check` passed in both repos (Git only reported LF/CRLF conversion warnings). A read-only MOLDRISE v1.0.5 extraction using D: scratch storage succeeded with the counts above.
- Main repo: `go test ./... -count=1`, `go vet ./...`, and `go build ./...` passed after the current code changes; frontend lint, `npx tsc --noEmit`, and `npm run build` passed.
- Generic injector follow-up: a source review found it accepted a staged file when translated keys were missing or duplicated. It now rejects both cases; `go test ./pkg/inject -count=1` and the full Go test/vet/build checks pass.
- Tools repo: `go test ./... -count=1`, `go vet ./...`, and `go build ./...` pass after Smaz and Godot `.translation` work. Two synthetic fixtures and real 269-entry FA/EN resources were read, rewritten, and loaded by Godot 4.5.1. The main extractor export step mapped 4,842 entries across 18 locales; injector/PCK repack integration and other resource-version fixtures remain open.
- Patcher was rebuilt with `pwsh -File patcher/build.ps1` during this pass. Rebuild it if C# changes.
- `TestPatcherV2RollsBackEarlierTargetsWhenLaterTargetFails` passed; it verifies prior-target restoration and cleanup after a later copy-from-base failure.
- Deterministic apply and uninstall race tests change the target from the existing log callback after preflight; post-move hash checks reject the change, preserve it, restore backups, and clean `.ffnew` files. This covers the swap interval without a public failure switch.
- FFP1 package building rejects duplicate case-insensitive targets and Windows-special path components; the C# reader rejects duplicate aliases, alternate data streams, reserved device names, invalid Windows characters, and ambiguous trailing dot/space components. Focused tests pass.
- Apply uses exclusive `.ffnew` creation and rollback only deletes files matching the patch's final hash; integration tests prove concurrent target edits and unowned `.ffnew` files survive a later-target failure.
- `graphify update .` completed after the code edits.
- No broad real-game injection validation was performed in this pass. Existing real-game extraction counts in older memory are historical evidence and need revalidation before being presented as current.
- Current main-repo working tree contains intentional edits plus untracked `Tools.rar`; do not stage or remove `Tools.rar` as part of this work.
- Current real Godot test output is under `%TEMP%\FarsiForge-validation-godot-20261009\`; it is outside the game directory. Keep it until any needed metrics/fixture work is complete; do not copy proprietary game assets into the repository.
- The `D:\games` detection sweep returned labels for all 14 top-level folders: SAGE (2), GoldSrc (1), custom (2), FromSoftware (1), Factorio (1), Unreal (3), Godot (1), Unity (2), and Project Zomboid (1). Detection results do not prove extraction/write-back.
- Factorio extraction project is under `%TEMP%\FarsiForge-validation-factorio-20261009\`.



