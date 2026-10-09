# Progress

## Scope Directive (user, 2026-10-09)
FarsiForge's goal is detecting and extracting text from **ALL games**, not a fixed set. GTA V / ELDEN RING were only test cases for understanding engine families. New extractors must be engine-family-generic (RPF7 → RAGE family; BHD5/DCX/FMG → all FromSoftware titles with version switches), fitting the existing pkg\detection + pkg\extract registry architecture.

## Task 4 — Phase 1 Engine-Family Extractors (IN PROGRESS, 2026-10-09)
**Definition (user-confirmed via question tool)**: Task 4 = "Proposal B" = Phase 1 extractors — native Go readers for the two new engine families, extraction only (no injection), validated on real installs:
- **RAGE family** (GTA V test case): `pkg\extract\rpf7.go` (RPF7 archive reader: NG-encrypted TOC, Deflate, nested .rpf recursion) + `pkg\extract\gxt2.go` (GXT2 text: jenkins-hash keyed, UTF-8).
- **FromSoftware family** (ELDEN RING test case): `pkg\extract\bhd5.go` (BHD5/BDT: RSA-encrypted header, 64-bit ER file headers, per-file AES-ECB) + `pkg\extract\dcx.go` (DCX decompression: Oodle via game's oo2core_6_win64.dll + zlib) + `pkg\extract\fmg.go` (FMG v2, UTF-16LE; BND3/BND4 container ~150 lines).
- Both wired into `pkg\extract\engines.go` registry as engine-family-generic strategies (RAGE → any RPF7 title; FromSoftware → version-switched BHD/DCX/FMG).

### Todo list (9 items, live state)
1. **[PARTIAL]** Crypto gathering — GTACrypto.cs confirms RPF7 NG crypto depends on 101 executable-derived keys and 17×16×256 decrypt tables; implementing GTA5.exe hash harvesting remains open. ELDEN RING Data0 RSA public key was validated against the real local archive.
2. **[PENDING]** Implement `pkg\extract\rpf7.go` — RPF7 tree parsing, NONE/AES/NG TOC decrypt, Deflate/resource payloads, nested archives. All probed GTA V RPFs use NG encryption, so real extraction depends on item 1.
3. **[PARTIAL]** `pkg\extract\gxt2.go` implements a bounds-checked standalone GXT2 UTF-8 table parser and JOAAT hash. Reading GXT2 files out of RPF archives and checking the real american.gxt2 count remain open.
4. **[PARTIAL]** `pkg\extract\bhd5.go` implements ER RSA block decryption, BHD5 64-bit index parsing, ER path hash, and bounded random-access BDT record/AES-range reading. Live Data0 validation recovered 5,824 entries. Data1-3/DLC RSA keys and non-ER BHD variants remain open.
5. **[DONE for ER]** `pkg\extract\dcx.go` handles zlib frames and ER KRAK DCX via `oo2core_6_win64.dll`; `oodle_windows.go` calls the game DLL. Live decoding produced a BND4 payload. Other DCX variants need further coverage.
6. **[PARTIAL]** `pkg\extract\fmg.go` parses FMG v1/v2 (wide UTF-16) and `pkg\extract\bnd4.go` traverses common BND4 message bundles. Real FMGs are extracted. BND3 support remains open.
7. **[PARTIAL]** Registered `RAGEExtractor` and `FromSoftwareExtractor` in the default extraction registry. ER Data0 extraction works; RAGE encrypted archive extraction and broader FromSoftware coverage remain open.
8. **[PARTIAL]** Live ELDEN RING validation extracted 520,750 entries from 24 message bundles via Data0 BHD5/BDT, AES, KRAK/Oodle, BND4, and FMG. GTA V real archives are NG-encrypted and still cannot be traversed.
9. **[PARTIAL]** Parser unit tests pass. `go build ./...`, `go vet ./...`, and `go test ./...` passed; after final bounds checks, `go test ./pkg/extract` passed. `graphify update .` passed. Task 4 remains incomplete.


### Constraints/decisions for Task 4
- GPL-3 sources (CodeWalker, SoulsFormatsNEXT, WitchyBND) → port documented formats, NOT code. GTACrypto.cs is MIT (Neodymium 2015) but still port the format, not the code.
- Native Go estimate ~1,500 lines (stdlib + game's Oodle DLL). Injection (RPF space management / BDT-BHD repack) = Phase 2, NOT Task 4.
- All probed GTA V archives are NG-encrypted; full RAGE extraction requires NG key/table discovery from the matching executable.
- SoulsFormatsNEXT FMG.cs SHA 443d36e4f88ff0f689cbd709affbbe1de4c2ce55; WitchyBND key SHA 28f6af34bdc565aed714074129e5c697b1a58c8b (file path still unknown).

## What Works (verified)
- **Task 1 — bundle/portability verification DONE (2026-10-09, commit 5763385)**: audit found 3 hardcoded-path violations in cmd entrypoints; fixed: cmd\farsiforge-cli\main.go + cmd\farsiforge\main.go now call `tools.NewRegistry("")` (portable resolution: exe-relative → dev walk-up → fallback), and handleBuildInstaller's hardcoded `D:\FarsiForge\output\<game>` default replaced with `<gameRoot>\FarsiForge_Patch` mirroring the Wails app's BuildPatcher (app.go:348). COMPLIANT (no change needed): pkg\tools\tools.go registry resolution, pkg\tools\embed.go (go:embed all:scripts), all 4 app.go call sites, patcher\build.ps1. Acceptable: pkg\core\config.go defaultGameSearchPaths (dirExists-filtered candidate list, not a tool path). NOTE: Tools\ is gitignored with no packaging script — bundling gap to address in Task E/F. `go build/vet/test ./...` green; graphify updated.
- **Task 3 — GTA V / ELDEN RING research DONE (2026-10-09)**: full report at memory-bank\research-gta5-eldenring.md. Verified on real installs: GTA V = RPF7 (magic 0x52504637, ALL archives NG-encrypted 0x0FEFFFFF) + Deflate + trivial GXT2 text format (UTF-8, jenkins-hash keyed); ER = RSA-encrypted BHD5 headers (public-key PEM known), 64-bit ER file headers, per-file AES-ECB ranges, path hash i*0x85+c, DCX via game's own oo2core_6_win64.dll, MSGBND→FMG (UTF-16LE, version-2 wide layout). Both feasible as native Go readers (~1,500 lines total, stdlib + game's Oodle DLL); injection = Phase 2 (RPF space management / BDT-BHD repack). Reference: CodeWalker + SoulsFormatsNEXT/WitchyBND (GPL-3 — port from documented formats, not code).
- **Task 5 — search feature (2026-10-09)**: `Project.SearchEntries(query, status)` in pkg\core\project_methods.go — case-insensitive substring across ID/Source/Translation/File/Notes; status filter accepts status values or pseudo-status "qa" (entries with QA notes). Surfaces: Wails binding `App.SearchEntries(gameRoot, query, status)` (app.go), CLI `-action search -query ... -status ...` (loads saved project, prints first 50 matches with ⚠ notes), frontend translate step (search box + status dropdown + "نمایش X از Y" counter + 200-entry render cap + amber QA-notes badge). Also fixed pre-existing frontend bugs: controlled input read `item.translation` but wrote `item.Translation` (typing never displayed), and `[...translations]` shallow-copy mutated state objects — now immutable idempotent updates via original index. Tests: pkg\core\search_test.go (11 assertions). `go build/vet/test` + `npx tsc --noEmit` all green.
- **Task Q — textfilter QA package (2026-10-09)**: new `pkg\textfilter` — markup tokenizer (placeholders `{0}`, tags `<cf>`, brackets `[x]`) + `QA(source, translation) []string` issue codes (EMPTY_TRANSLATION, MISMATCH_PLACEHOLDERS/TAGS/BRACKETS with missing/extra tokens, *_ORDER_DIFFERS, UNBALANCED_BRACES, LINE_COUNT_MISMATCH) + `QANotes()` "; "-joiner. Wired into `Project.SetTranslation` + `ImportTranslations`: every translation write records QA findings in `StringEntry.Notes` (flows to Wails frontend via project JSON; empty translation flagged EMPTY_TRANSLATION). Tests: pkg\textfilter\textfilter_test.go (10 want-true + 6 want-false rows) + pkg\core\qa_test.go. `go build/vet/test ./...` all green.
- **Extraction noise-filter fixes (2026-10-09, verified on real games)**: uasset.go/engines.go/engines_classic.go/godot_text.go filters (path/filename prefix-only, engine-noise words → neutral, asset-extension list removed). Hacker Simulator: 11,658 → 11,676 entries (+16 legit restores, 0 suspicious removals); MOLDRISE baseline→new3: +134 added / 22 removed / 0 suspicious; new2→new3 byte-identical (fixes are pure loosening). Garbage detector on new3-HS: same 8 by-design survivors, no new garbage.
- **Task A — pkg\tools overhaul**: silent exec (proc.go / proc_windows.go / proc_other.go, CREATE_NO_WINDOW 0x08000000), embed.go, rewritten tools.go (NewRegistry portable, RunSilent/RunSilentTimeout, WriteScript, ListScripts), proc_test.go. `go build ./...` passes.
- **Task B — Python scripts rewrite**: `Tools\UnityPy\extract.py` (9,802B) + `inject.py` (11,355B); py_compile passes. NOTE: `pkg\tools\scripts\extract.py` is STALE (3,079B) — go:embed serves old copy; Task U re-syncs byte-identical.
- **Task D — farsiforge-tools repo**: D:\FarsiForgeTools, module github.com/Amirwopi/farsiforge-tools, zero deps; fftools with locres export/import (v0–v3), text extract/inject, pck list/extract (v2/3/4). Pushed. Deployed to `D:\FarsiForge\Tools\fftools\fftools.exe`.
- **Task P — C# GUI patcher**: `patcher\FarsiForgePatcher.cs`, build.ps1, make_test_patch.py, FORMAT.md; exes in `patcher\bin\` + `Tools\patcher\`; pkg\ffpatch Go writer; Go↔C# cross-validation incl. uninstall roundtrip passes.
- **Task V — real-game validation**: 18-dir engine matrix (EscapeTheBackrons=Unreal, ASKA=Unity IL2CPP, MK10=UE3); fftools pck NUL bug fixed+pushed; locres Persian roundtrip OK; Machine Party pck v3 extracts 4,217 files.

## What's Left
- **Fortnite (0 strings)**: chunked .pak + .utoc/.ucas (UE5.6 Zen) — locres inside chunked paks; needs repak utoc support or FModel.
- **CS2 (0 strings)**: text inside .vpk files — needs Valve VPK reader in fftools or extract package.
- **Brawlhalla (custom)**: not yet supported — would need a detector + extractor.
- **ELDEN RING**: Data0 message extraction is implemented and locally verified (520,750 entries across 24 bundles); other BHD variants and BND3 coverage remain open. **GTA V**: RPF7 NG-encrypted archives still need traversal and executable-derived keys/tables. **MK10**: archive text extraction remains open.
- **MOLDRISE injection**: extraction works via gdre recovery; injection needs gdre --bin-to-txt + txt-to-bin or --pck-patch.
- **Orc Massage injection**: inject.py has no raw-mode path (typetrees broken).
- **G2 — Godot translation support in fftools** (NOT started).
- **repak automation**: manually extracted to Tools\repak\; wire into tools.go scan() + GetPath("repak"). gdre_tools IS already discovered.
- **Shared work-dir cache warning**: %TEMP%\farsiforge_work shared across games — per-game work dirs would avoid stale typetree caches.
- **Task U (Unity IL2CPP typetree reconstruction)** still pending for IL2CPP games — but ASKA already extracts 135k via raw-scan Strategy 5.
- **Task E/F** (frontend settings page, bundling/LFS) — future.

## Current Status
- **v1.1.0 RELEASED 2026-10-09**: commit 5c95078 pushed to origin/main (Amirwopi/FarsiForge), tag v1.1.0 pushed. User-authorized. 34 files (11-engine detection + extraction sweep + patcher/ffpatch + README).
- **Post-release commits (2026-10-09, local only, NOT pushed)**: d9b89c3 = textfilter QA package + Notes wiring + extraction noise-filter fixes; 8920c83 = Task 5 search feature (core SearchEntries + Wails binding + CLI action + frontend search/filter/QA badge + editor bug fixes).
- **Remaining tasks**: Task 4 is in progress: ELDEN RING Data0 extraction is verified; RPF7/NG traversal and GTA V real text-count validation remain, along with broader FromSoftware format coverage.
- **Detection DONE**: 11 detectors — Unity, Unreal, UE3, Godot, SAGE, GoldSrc, Source, Source2, FromSoftware, Factorio, Zomboid, RAGE + Generic fallback + nested-dir resolver.
- **Extraction DONE**: Unity (UnityPy + raw-scan fallback), Unreal (.locres loose + pak + .uexp batch), Godot (gdre recovery + text parsers), GoldSrc/Source2 (KeyValues), Zomboid (Lua), Factorio (cfg), SAGE (.big/.csf/.str/.manifest).
- **Critical fix shipped**: embed.go filepath.Join→path.Join (all Python-script extraction was broken).
- **UnrealExtractor perf fix shipped**: batch .uexp unpack + pak-locres (EscapeTheBackrooms: 1.5h→34s).
- repak at Tools\repak\repak.exe (manual; not in tools.go scan() yet).
- UnityPy 1.25.4 installed; Tools\Il2CppDumper\ exists.

## Detection Matrix (2026-10-09, D:\games + D:\SteamLibrary\steamapps\common)
| Title | Engine | Conf |
|---|---|---|
| CnC Zero Hour - R2P Edition | sage | 95% |
| Command and Conquer Red Alert 3 | sage | 95% |
| Counter-Strike 1.6 | goldsrc | 95% |
| ddper | custom | 10% (custom C++, honest) |
| ELDEN RING | fromsoftware | 95% |
| Factorio | factorio | 95% |
| farsisaz | custom | 10% (empty dir) |
| Fortnite | unreal | 86% |
| Hacker Simulator | unreal | 86% |
| MOLDRISE.v1.0.5 | godot | 90% |
| Orc Massage | unity | 95% |
| Project.Zomboid | zomboid | 95% |
| Raft | unity | 95% |
| Riot Games | unreal | 43% (launcher-only dir) |
| ASKA | unity | 95% (il2cpp) |
| Brawlhalla | custom | 10% (not yet supported) |
| CS:GO/CS2 | source2 | 80% |
| Deadlock | source2 | 80% |
| EscapeTheBackrooms | unreal | 100% |
| GTA V | rage | 95% |
| MK10 | ue3 | 71% |
| Supermarket Together | unity | 95% (mono) |
| VPet | custom | 10% (empty/incomplete install) |
| Steamworks Shared | custom | 10% (not a game) |

## Extraction Results (2026-10-09, verified via CLI)
- Raft (Unity Mono): 58,368 ✓ · Orc Massage (stripped Unity): 607 ✓ (raw-scan Strategy 5)
- Hacker Simulator (UE4): 11,139 ✓ (pak .locres + Data_Tables/Widgets .uexp batch)
- EscapeTheBackrooms (UE5): 103,070 ✓ (Game.locres from pak + Items .uexp)
- MOLDRISE (Godot): 5,980 ✓ (gdre recovery + .tres/.gd/.tscn parser)
- Supermarket Together (Unity): 14,203 ✓ · ASKA (Unity IL2CPP): 135,788 ✓
- Deadlock (Source 2): 57,874 ✓ · CS 1.6 (GoldSrc): 2,664 ✓
- Project Zomboid: 14,220 ✓ · Factorio: 14,945 ✓
- CnC Zero Hour (SAGE): 1,532 ✓ · RA3 (SAGE): 34,714 ✓ (csf + manifests)
- ddper (custom, generic extractor): 504 ✓ · ELDEN RING (generic txt): 1,444 ⚠
- Fortnite: 0 ⚠ (chunked paks; locres not yet reachable) · CS2: 0 ⚠ (text in VPK, not extracted yet) |

## Extraction Results (2026-10-08)
- **Raft (Unity Mono)**: 58,368 strings ✓ (was 37,879 — stale typetree cache in shared work dir was capping reconstruction)
- **Hacker Simulator (UE4)**: 132,495 strings / 830 files ✓ (text hardcoded in .uasset/.uexp DataTables+Widgets, NOT .locres; read via `repak get` per .uexp)
- **Orc Massage (stripped Unity)**: FIXED 2026-10-08 → 607 strings / 5 files ✓. Root cause: no Managed/, no GameAssembly.dll; 11,479/12,151 MonoBehaviours have corrupt/stripped typetrees (read_typetree fails "Expected X bytes, only read Y"; even check_read=False yields 0/2000). Fix: extract.py Strategy 5 "raw-data string scan" — scan MonoBehaviour raw serialized bytes for printable UTF-8 runs, strict filters (space + 2 consecutive letters + alnum ratio >= 0.65, reject paths and "Class, Assembly-CSharp" refs), skip globalgamemanagers* engine files. Text confirmed real: "START MASSAGE", "Essential Oil Table", "White Elf", "Sure To Skip", dialogue "A~~ You're hurting me on purpose, right?".
- **MOLDRISE (Godot)**: FIXED 2026-10-08 → 5,980 strings / 547 files ✓. Root cause: no .translation/.csv/.po — all text hardcoded in binary .res (dialogue graphs), .scn (scenes), .gdc (scripts). Fix: GodotExtractor now runs `gdre_tools --headless --recover=<pck> --output=<workDir>\gdre_recovered` (converts .res→.tres, .scn→.tscn, .gdc→.gd, 23s for 376MB pck), then parses recovered text files with new pkg\extract\godot_text.go (extractGodotTextStrings: double-quoted literal scan + godotUnescape + isGodotText filter — rejects paths/uids/bbcode/identifiers like DEMO_DIALOG_54, dialogue_node_54, npc.sister.name, GodotSteam; keeps dialogue + UI words like Install/Cancel). Fallback to old fftools pck-extract path if gdre missing/fails. Unit tests in pkg\extract\godot_text_test.go. Sample output: "You should visit Dr. Lee. She is a good soul and helps anyone if you ask nicely.", "Go find her. Eighth floor. And come back and tell me how it goes, alright?"

## Known Issues / Blockers
- Unity typetree stripping (root cause of 87–99.6% skipped assets) — fix in flight (Task U).
- No "reconstructed"/TypeTreeGenerator references in code yet — Task U introduces them.
- Unreal .pak extraction blocked on Oodle (oo2core) — deferred.
- Git LFS/bundling strategy undecided (Task F).
