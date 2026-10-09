# Active Context

## Project Goal
FarsiForge aims to detect games across engine families, extract translatable text, support Persian translation and validation, inject translations where format support exists, and build installable/reversible patches. Engine support must be reported per capability; detection does not imply extraction or injection support.

## Repository Map
- Main app: pkg/detection, pkg/extract, pkg/inject, pkg/core, pkg/installer, pkg/ffpatch, pkg/validate, Wails API in app.go, frontend in frontend/.
- External Go tools: D:\FarsiForgeTools (fftools), consumed from Tools/fftools/; separate module and release lifecycle.
- Standalone patcher: patcher/FarsiForgePatcher.cs, reads FFP1 produced by pkg/ffpatch.
- Local-only game samples and tool payloads are ignored by Git. Do not delete ignored or untracked artifacts as generic cleanup.

## Current Work
A repository-wide quality audit is in progress across both Go projects, the UI, and the patch pipeline. Confirmed updates in this pass:
- Removed one-off Python scripts that wrote directly to an absolute developer checkout path.
- Removed the sample game path from the UI and disabled detection until a folder is selected.
- Made project-root detection match the exact Go module name and search from the executable as well as the working directory.
- Replaced drive-specific game search defaults with existing Steam/home folders plus environment overrides.
- Removed date/version-specific tool executable names; Python discovery now honors FARISIFORGE_PYTHON.
- Added FFP1 target path validation, installer input checks, and error propagation for patch finalization/readme/font staging.
- Hardened the C# patch reader against unsafe paths, malformed strings, invalid record sources, and out-of-range payload references.
- Hardened fftools PCK/text output paths and added regression coverage in the separate tools checkout.

## Validation State
Final audit checks passed: go test ./..., go vet ./..., go build ./..., golangci-lint run ./pkg/..., frontend lint/typecheck/production build, patcher C# build, FarsiForgeTools test/vet/build, and graphify update. The separate tools checkout contains a pre-existing untracked internal/translation/smaz.go; preserve it.

## Known Support Limits
- FromSoftware extraction is verified for ELDEN RING Data0 message bundles; broader BHD keys, BND3, and other DCX variants remain incomplete.
- Godot .translation export/import commands are absent; recovered text resources still work. RAGE has a standalone GXT2 parser; RPF7 NG archive traversal and GTA V text extraction remain incomplete.
- Injection coverage is narrower than detection/extraction: engine-specific injectors currently cover Unity and Unreal plus a generic path. Godot write-back explicitly reports unsupported. Other engine families need format-specific write support and validation.
- .locres real-game validation and several proprietary/packed formats still need sample-based verification.

## Next Actions
1. Extend real-game fixtures for formats still marked pending.
2. Preserve unrelated local artifacts and keep README capability claims evidence-based.
3. Continue implementing RPF7 and Godot .translation readers/writers.