# Project Progress

Updated: 2026-10-09
Overall: Active development; support differs by engine and operation.

## Product Objective
Build a dependable Persian localization tool that can detect game technologies, extract text, support translation QA and Persian shaping, inject where formats allow, and produce safe reversible patches. Keep engine-specific behavior behind the detection/extraction/injection registries. Do not describe an engine as supported for a capability until verified on representative game data.

## Architecture
- pkg/detection: detector registry and engine identification.
- pkg/extract: engine extractors and format readers.
- pkg/core: project model, configuration, shared interfaces.
- pkg/textfilter, pkg/validate: translation QA and validation.
- pkg/persian: shaping, bidi, digit and punctuation handling.
- pkg/inject: write-back strategies.
- pkg/ffpatch, pkg/installer, patcher/: FFP1 creation, package staging, apply/rollback/uninstall.
- pkg/tools: portable dependency discovery and process execution.
- frontend/, app.go: Wails desktop UI and bindings.
- D:\FarsiForgeTools: independent fftools module for .locres, Godot .pck, and generic text operations.

## Verified Capabilities
- Detection registry covers Unity, Unreal, UE3, Godot, SAGE, GoldSrc, Source 2, FromSoftware, Factorio, Project Zomboid, RAGE, and generic fallback.
- Extraction is verified for Unity, Unreal localization assets, Godot resources, Valve localization, Factorio, Project Zomboid, and SAGE families; see the engine matrix and CLI validation records for per-game counts.
- Local ELDEN RING Data0 validation extracted 520,750 entries from 24 message bundles through BHD5/BDT, AES, DCX/Oodle, BND4, and FMG.
- Godot .translation CLI export/import is not implemented; recovered text resources are supported. The RAGE GXT2 parser exists, but RPF7 traversal is not implemented. GTA V NG-encrypted archives are not extracted.
- FFP1 has Go writer/C# reader cross-validation coverage. The patcher supports apply, rollback, and uninstall.
- Translation search and QA notes are wired through project methods, Wails/CLI, and the translation UI.

## Active Quality Work
- Eliminate developer-machine absolute paths and version/date-specific executable names. Use executable-relative discovery, OS environment variables, and user configuration.
- Keep archive and translation file paths confined to caller-selected roots.
- Propagate write, close, staging, and packaging errors; do not report success for partial outputs.
- Keep UI state typed and generated bindings excluded from linting.
- Clean obsolete scripts only after verifying they are not called or documented.

## Current Limitations
- RAGE RPF7 NG key/table discovery and nested archive traversal are open.
- FromSoftware support beyond the tested ELDEN RING Data0 variant is incomplete; BND3 and additional DCX variants are open.
- Injection is implemented for Unity, Unreal, and generic text paths. Godot and other detected engines require format-specific write support; Godot currently returns an explicit unsupported error.
- Some format support remains synthetic-test-only. In particular, .locres needs shipped-game validation and packed formats need representative sample tests.
- Patch packaging should remain streaming and recoverable for very large assets; validate crash/partial-output behavior before claiming production-grade reliability.

## Verification Record
- Final main-repository checks passed: go test ./..., go vet ./..., go build ./..., and golangci-lint run ./pkg/....
- Earlier extractor implementation had real ELDEN RING validation and graph refresh.
- Frontend ESLint, TypeScript, and production build passed. The C# patcher compiled and Go-to-C# patch cross-validation passed.
- FarsiForgeTools test, vet, and build passed after archive/injection path validation. fftools is Go 1.21 with standard library only; main FarsiForge uses Go 1.26 and Wails.

## Decisions
- Generalize readers by engine family and version; GTA V and ELDEN RING are validation fixtures, not the product scope.
- Injection is separate from extraction; do not imply extraction support guarantees safe write-back.
- Third-party format references may guide independent implementations; do not copy GPL source into this project.
- User-specific game/tool directories are local data and are not to be removed during code cleanup.

## Next
1. Close out the current cross-project audit and record actual results.
2. Add real-game fixtures for .locres, RPF7/GXT2, Godot .translation resources, and non-ER FromSoftware variants.
3. Implement Godot .translation export/import and extend safe injection per engine; test patch install, rollback, and uninstall against disposable game copies.
4. Update README engine capability claims from measured evidence.