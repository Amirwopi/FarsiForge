# Project Progress

Updated: 2026-10-09

## Objective

Build an evidence-driven Persian game-localization workbench: recover player-facing text with stable context, write supported formats correctly, and deliver reversible patches. Track detection, extraction, injection, archive rebuild, and installation as separate capabilities.

## Implemented foundations

- Project JSON and work outputs are stored outside game directories. Re-extraction preserves translations only when entry identity matches; legacy project files are copied and retained.
- Translation saves now validate stable IDs and status values before mutation, rejecting duplicate project/submission IDs and stale IDs from an old UI view instead of silently dropping or ambiguously overwriting changes.
- Translation carry-over keys are represented as a comparable tuple of container, file, key, context, and source fields; delimiters inside source data cannot create false identity matches.
- Main Go application includes engine/format registries, staged injectors, Persian text processing and QA, FFP1 patch construction, and a C# installer with original-file checks, backup, rollback, and uninstall.
- FFP1 v2 binds replacement targets to original size and SHA-256 while retaining v1 read compatibility. Recent patcher changes cover reparse-point escapes, concurrent file changes during apply/uninstall, stale temporary files, duplicate target aliases, and rollback behavior.
- `D:\FarsiForgeTools` is a separate Go module. Its Godot tooling reads/writes supported Godot 4 `OptimizedTranslation` resources and rebuilds standalone unencrypted PCK v2/v3/v4. CSV catalogs are required to resolve translation hash IDs to exact source keys.
- Main Godot extraction and injection retain PCK identity. Injection currently updates existing Persian `.translation` resources only when the matching CSV and PCK are available; the rebuilt full PCK is staged for patch creation.
- Generic UTF-8 key/value injection now fails on absent or ambiguous translated keys instead of silently claiming a complete write.
- Staged patch output is bound to SHA-256 hashes captured after successful injection. `BuildPatcher` passes the expected hash to the installer, which validates before creating output and rechecks while writing; a changed game file requires reinjection. Full Go tests, vet, build, and `git diff --check` passed after this hardening.
- Project save replacement now has a Windows failure-path test: it holds the existing project file without delete sharing, verifies replacement fails, confirms the previous JSON bytes remain intact, and checks the temporary file is removed. The successful replacement test also passes.

## Validation evidence

- Latest main-repository checks after the synthetic installer integration, translation-save validation, and structured re-extraction identity changes: `go test ./... -count=1`, `go vet ./...`, and `go build ./...` passed. The opt-in Godot integration test also passed with a Godot-generated disposable PCK, local `fftools`, and the C# patcher. `graphify update .` completed.
- Synthetic Godot roundtrip covered exact CSV/hash identity, translation import, PCK rebuild, FFP1 build, C# apply, and uninstall. Apply installed the rebuilt PCK; uninstall restored original bytes. Godot 4.5.1 previously loaded the rebuilt pack and returned the changed Persian message. This is not a real-game validation.
- A local Godot 4.3+ / PCK v3 sample yielded 8,123 strings across 390 recovered files (7,014 `.tscn`, 1,024 `.gd`, 85 `.tres`). Separately, a recovered project mapped 4,842 entries across 18 locale resources; 4,826 had non-empty existing translations. Counts do not establish semantic completeness.
- Read-only MOLDRISE v1.0.5 validation: Godot PCK v4 / engine 4.7.0, 4,620 packed files. GDRE recovery plus the main pipeline extracted 3,789 strings from 467 `.gd/.tscn/.tres` files. It originally included 2,280 entries from add-ons, of which 1,812 were under editor directories; the extractor now excludes those editor files and quoted dictionary keys. The resulting project has 3,789 unique IDs, non-empty literal paths/context, and source line numbers; 413 runtime/add-on entries remain. Dialogue paragraphs and menu/accessibility copy were inspected and confirmed as authored strings. Item/note coverage is incomplete: inspected item paths yielded mostly state labels and a `newspaper` name, and image/texture text is outside this extractor.
- Read-only Factorio extraction yielded 14,945 entries from 53 `.cfg` files with no missing/duplicate IDs. DDPER 8.4 generic fallback yielded 504 entries from 55 `.json`/`.txt` files; semantic coverage and write-back remain unverified.
- Read-only Unity 2021.2.3 / Orc Massage extraction yielded 607 entries from 5 files (383 `.assets`, 224 extensionless levels), with unique non-empty IDs. No write-back or game-load check was done; use a bounded disposable fixture rather than copying the full installation.
- Follow-up audit of that same read-only Unity project found all 607 entries use the `raw_*` fallback path. It includes genuine dialogue/UI text and controls, but also repeated placeholder strings, Unity asset-store documentation, and UI Toolkit style declarations. Raw entries lack verified typetree field identity and are not injectable with the current importer; Unity injection now fails before invoking tools when these entries are translated, and the UI marks them as non-injectable candidates. This is a conservative failure path, not Unity write-back support. Full Go tests, vet, build, frontend lint, TypeScript check, and diff check passed after the guard/UI change.
- A scan of 14 `D:\games` folders reported SAGE, GoldSrc, custom, FromSoftware, Factorio, Unreal, Godot, Unity, and Project Zomboid labels. Detection results are not extraction or injection proof. Older ELDEN RING counts are historical and need revalidation.

## Limits and open work

- No real shipped Godot game PCK has passed write-back and gameplay validation. Godot embedded/encrypted/sparse packs, scripts/scenes, and creation of a missing Persian locale remain unsupported.
- MOLDRISE's PCK contains no `.translation`/CSV catalog, so the existing main injector cannot write back its recovered `.gd/.tscn/.tres` literals. A 4.7.0-compatible disposable runtime test is still required before claiming this game can be patched.
- Unity and Unreal write-back require real versioned validation; Unreal `.locres` support does not establish `.pak` rebuilding.
- RAGE encrypted RPF7 traversal is not implemented; loose GXT2 support is not GTA V archive support. Broader FromSoftware variants and safe injection are also open.
- Player-facing semantic coverage (dialogue, UI, journals, notes, items, tutorials, quests), placeholders/markup, fonts/glyphs, and omissions need asset-family review.
- Consult root `todo.md` for the detailed work queue, current constraints, and next steps. Do not infer compatibility beyond the evidence above.

## Operating decisions

- Treat real game installations as read-only; use disposable copies or temporary output directories.
- Never use generic text mutation as fallback for an unknown engine/format.
- Preserve unrelated working-tree files and separate-repository work. Do not push the main repository without a direct request.
