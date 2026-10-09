<div align="center">
  
# 🇮🇷 FarsiForge (فارسی‌فورج)

**A Persian game localization workbench with format-specific extraction and reversible patch building**

[![Go Version](https://img.shields.io/badge/Go-1.26%2B-blue.svg?style=flat-square)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-blue.svg?style=flat-square)](https://reactjs.org/)
[![Wails](https://img.shields.io/badge/Wails-v2-red.svg?style=flat-square)](https://wails.io)
[![License](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)](LICENSE)

![FarsiForge Banner](https://via.placeholder.com/800x200.png?text=FarsiForge+Persian+Game+Engine)

*FarsiForge helps locate game text, manage Persian translations, stage supported file changes, and build a patch package. Support depends on the engine, file format, and game version; detection does not guarantee extraction or write-back.*

</div>

## ✨ Features

- **🚀 Engine detection:** Identifies multiple engine families; each engine has its own extraction and write-back limits in the matrix below.
- **📝 Text extraction:** Format-specific readers cover selected Unity assets, Unreal localization data, recovered Godot text resources, Valve localization files, Lua tables, and `.big/.csf` data.
- **🔄 Persian Text Shaping:** Built-in RTL (Right-to-Left) reshaping, bi-directional text processing, and Arabic-Indic digit conversion.
- **🎨 Desktop workflow:** Review entries, edit translations, run validation, stage supported changes, and build patch packages.
- **🛠️ Localization Workflow:** Detect, extract, review, validate, and build patches locally. Injection availability varies by engine and file format.
- **📦 Patch Generator:** Easily build final translation installers/patches with translator credits to share with the community.

---

## 🎮 Supported Engines

| Engine / family | Detection | Extraction | Injection / rebuild | Evidence status |
|--------|-----------|-----------------|---------------|---------|
| **Unity** (Mono + IL2CPP) | ✅ | UnityPy TextAsset and recovered MonoBehaviour paths; raw byte scans are marked as candidates | Staged write-back requires a verified writable typetree identity; raw-scan candidates are rejected | Write-back is not validated on a real shipped game; confirm per game, Unity version, and asset type |
| **Unreal Engine 4/5** | ✅ | `.locres` and related localization paths | Loose `.locres` staging path | Implemented; shipped-game roundtrip and `.pak` rebuild are not established here |
| **Godot 3/4** | ✅ | Recovered `.tres/.gd/.tscn` text and CSV-backed Godot 4 `.translation` resources | ⚠️ Existing Persian `.translation` resources in standalone, unencrypted PCK v2-v4 | Updates exact CSV-resolved messages and stages a rebuilt PCK. Does not translate scenes/scripts, add a new locale resource, or handle embedded, encrypted, or sparse packs. |
| **SAGE** (CnC Generals / ZH / RA3) | ✅ | `.big` / `.csf` reader paths | ❌ Not implemented | Extraction path; representative write-back not implemented |
| **GoldSrc** (Half-Life / CS 1.6) | ✅ | Valve KeyValues localization files | ❌ Not implemented | Extraction path; no injector |
| **Source 2** (CS2 / Deadlock) | ✅ | Valve KeyValues localization paths | ❌ Not implemented | Extraction path; no injector |
| **FromSoftware** | ✅ | BHD5/BDT, DCX, BND4, FMG paths; focused on ELDEN RING Data0 | ❌ Not implemented | Limited extraction scope; other games/variants need validation |
| **Factorio** | ✅ | `locale/*.cfg` paths | ❌ Not implemented | Extraction path; no injector |
| **Project Zomboid** | ✅ | Lua translation table paths | ❌ Not implemented | Extraction path; no injector |
| **RAGE** (GTA V) | ✅ | Loose GXT2 parser | ❌ Not implemented | Encrypted RPF7 traversal is not implemented |
| **UE3** (MK10 etc.) | ✅ | Limited Coalesced/`.upk` detection/read paths | ❌ Not implemented | Partial; requires game-specific validation |

Detection, extraction, archive rebuild, injection, and patch installation are separate capabilities. A detected engine may not support extraction, and an extracted format may not support safe write-back. Do not treat this matrix as a guarantee for every title or version.

Historical local extraction runs recorded per-game counts, including an ELDEN RING Data0 run of 520,750 entries across 24 message bundles. Those results have not all been revalidated against the current tree and game builds; treat them as historical measurements, not a compatibility guarantee. GTA V encrypted RPF7 extraction is not implemented.

Current local fixture runs provide limited extraction evidence:

- A read-only MOLDRISE v1.0.5 sample (PCK v4 / Godot 4.7.0) recovered 3,789 strings from 467 `.gd/.tscn/.tres` files. Editor add-on files and quoted dictionary keys are filtered; dialogue and menu/accessibility text were confirmed, while full item/note coverage and remaining false positives are still open. This PCK has no `.translation` catalog, so it does not validate injection. Separately, Godot 4.3+ / PCK v3 recovered 8,123 entries from 390 files (7,014 `.tscn`, 1,024 `.gd`, 85 `.tres`); a translation-export integration mapped 4,842 entries from 18 `.translation` resources against the local CSV. A synthetic Godot 4.5.1 PCK passed injection, FFP staging, and runtime loading. Real-game write-back and gameplay validation remain unverified.
- Factorio extracted 14,945 entries from 53 `.cfg` files. The saved project reported no missing or duplicate IDs.
- DDPER 8.4 was detected as a custom engine and the generic text path extracted 504 entries from 55 files (419 `.json`, 85 `.txt`) with no missing or duplicate IDs. Semantic completeness and write-back were not tested.
- Unity 2021.2.3 / Orc Massage extracted 607 entries from 5 files (383 `.assets`, 224 extensionless level files), with non-empty unique IDs. Audit found all 607 were raw-byte candidates with unknown field identity; examples included game dialogue/UI plus placeholders, asset-store documentation, and UI Toolkit style data. They are now labeled in the UI and rejected by Unity injection. No real Unity write-back or game-load test was performed.

These counts do not prove completeness or safe write-back for those games.

---

## 🏗️ Architecture

FarsiForge uses a **Go** backend and a Wails desktop shell. The repository contains a Next.js frontend under `frontend/`; confirm which frontend is wired into the target desktop build before making UI changes.
- **Backend (`pkg/`)**: 
  - `detection`: Identifies the engine families listed in the capability matrix; detection is not a support guarantee.
  - `extract`: Parses game assets via embedded Python scripts (Unity), native Go parsers (Valve KeyValues, Lua tables, Factorio cfg, SAGE `.big/.csf`), and CLI tools (repak, gdre_tools, fftools).
  - `inject`: Applies Persian processing and stages changes for implemented formats. This does not modify the installed game; Unity requires verified typetree fields, and Godot injection currently targets existing Persian `.translation` resources inside standalone, unencrypted PCKs and requires the matching CSV catalog.
  - `tools`: A smart registry that dynamically resolves required external dependencies (UnityPy, gdre_tools, repak, fftools).
- **Frontend (`frontend/`)**: Modern UI powered by TailwindCSS and Framer Motion.
- **Patcher (`patcher/`)**: Standalone C# GUI patcher (`FarsiForgePatcher.exe`, FFP1 format) that installs a translation patch directly onto the game.

---

## 🚀 Getting Started

### Prerequisites
- [Go 1.26+](https://golang.org/dl/)
- [Node.js 20.9+](https://nodejs.org/) (required by the installed Next.js version)
- [Python 3.10+](https://www.python.org/) with `pip`
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### Installation & Build

1. **Install Python Dependencies (For Unity)**:
   ```bash
   pip install UnityPy
   ```

2. **Build the Frontend**:
   ```bash
   cd frontend && npm install && npm run build
   ```

3. **Build the Desktop Application**:
   ```bash
   wails build -s
   ```
   *The final `.exe` is generated at `build/bin/FarsiForge.exe` — copy the `Tools\` folder next to it for a self-contained portable build.*

4. **Build the CLI**:
   ```bash
   go build -o build/farsiforge-cli.exe ./cmd/farsiforge-cli
   ```

---

## 💡 Usage Workflow

1. **Detect**: Open FarsiForge and select your game directory (e.g., `Supermarket Together`). The engine will be automatically detected.
2. **Extract**: The app unpacks data archives and extracts all translatable IDs into a local project file.
3. **Translate**: Use the built-in translation grid to edit strings.
4. **Stage changes**: FarsiForge processes translations for supported formats and writes staged files into the project workspace. It does not patch the installed game at this step.
5. **Build and apply**: Build a patch package. The separate patcher validates original files and applies the package with backups; validate a game/format combination before distributing a patch.

---

## 🤝 Contributing
Contributions, issues, and feature requests are welcome! 
Feel free to check the [issues page](../../issues).

## 📄 License
This project is licensed under the [MIT License](LICENSE).

<div align="center">
  <i>Developed with ❤️ for the Persian Gaming Community</i>
</div>
