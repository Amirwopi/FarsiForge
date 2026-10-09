<div align="center">
  
# 🇮🇷 FarsiForge (فارسی‌فورج)

**The Ultimate AI-Powered Persian Localization Framework for Modern Game Engines**

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg?style=flat-square)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-blue.svg?style=flat-square)](https://reactjs.org/)
[![Wails](https://img.shields.io/badge/Wails-v2-red.svg?style=flat-square)](https://wails.io)
[![License](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)](LICENSE)

![FarsiForge Banner](https://via.placeholder.com/800x200.png?text=FarsiForge+Persian+Game+Engine)

*FarsiForge is an automated, high-performance toolkit designed to extract, translate, shape, and inject Persian (Farsi) text into game assets across multiple engines like Unity, Unreal, and Godot.*

</div>

## ✨ Features

- **🚀 Multi-Engine Support:** 11+ engines — Unity (Mono/IL2CPP), Unreal 3/4/5, Godot 3/4, SAGE, GoldSrc, Source 2, FromSoftware, Factorio, Project Zomboid, RAGE.
- **📝 Intelligent Text Extraction:** Dialogue, item names, menus, tutorials and descriptions from `TextAsset`/`MonoBehaviour` (Unity), `.locres` + `.uasset/.uexp` (Unreal, incl. `.pak`-packed), recovered `.tres/.gd/.tscn` (Godot), Valve KeyValues, Lua tables, `.big/.csf` archives and more.
- **🔄 Persian Text Shaping:** Built-in RTL (Right-to-Left) reshaping, bi-directional text processing, and Arabic-Indic digit conversion.
- **🎨 Modern Dashboard:** A beautiful, GPU-accelerated desktop UI built with React, Next.js, and Wails.
- **🛠️ Automated Pipeline:** From extraction to injection, everything runs locally through a seamless step-by-step UI.
- **📦 Patch Generator:** Easily build final translation installers/patches with translator credits to share with the community.

---

## 🎮 Supported Engines

| Engine | Detection | Text Extraction | Tooling | Status |
|--------|-----------|-----------------|---------|--------|
| **Unity** (Mono + IL2CPP) | ✅ | ✅ TextAssets + MonoBehaviours + raw-scan fallback | UnityPy, Il2CppDumper | 🟢 Stable |
| **Unreal Engine 4/5** | ✅ | ✅ `.locres` (loose + inside `.pak`) + `.uasset/.uexp` scan | fftools, repak | 🟢 Stable |
| **Godot 3/4** | ✅ | ✅ `.pck` recovery → `.tres/.gd/.tscn` text + `.translation` | gdre_tools, fftools | 🟢 Stable |
| **SAGE** (CnC Generals / ZH / RA3) | ✅ | ✅ `.big` archives → `.csf` strings + manifests | built-in parser | 🟢 Beta |
| **GoldSrc** (Half-Life / CS 1.6) | ✅ | ✅ Valve KeyValues `*_english.txt` | built-in parser | 🟢 Beta |
| **Source 2** (CS2 / Deadlock) | ✅ | ✅ Valve KeyValues localization | built-in parser | 🟢 Beta |
| **FromSoftware** (ELDEN RING) | ✅ | ✅ ELDEN RING Data0 message bundles (BHD5/BDT → DCX/Oodle → BND4/FMG) | built-in Go readers + game Oodle DLL | 🟢 Beta (verified on local install) |
| **Factorio** | ✅ | ✅ `locale/*.cfg` | built-in parser | 🟢 Beta |
| **Project Zomboid** | ✅ | ✅ Lua translation tables | built-in parser | 🟢 Beta |
| **RAGE** (GTA V) | ✅ | ⚠️ Standalone GXT2 parser; RPF7 archive traversal is pending | built-in Go GXT2 parser | 🟡 Partial |
| **UE3** (MK10 etc.) | ✅ | ⚠️ Coalesced/`.upk` | — | 🟡 Detection only |

**Verified extraction results** (real games, 2026-10):
Raft 58k strings · Hacker Simulator 11k · EscapeTheBackrooms 103k · MOLDRISE 6k · Supermarket Together 14k · ASKA 135k · Deadlock 58k · Project Zomboid 14k · Factorio 15k · CS 1.6 2.7k · CnC Zero Hour 1.5k · RA3 35k · Orc Massage 607 · ddper 504 · ELDEN RING 520,750 entries (24 Data0 message bundles; local validation). GTA V RPF7 NG-encrypted extraction is not yet verified.

---

## 🏗️ Architecture

FarsiForge uses a robust **Go** backend integrated with a **React (Next.js)** frontend using **Wails**.
- **Backend (`pkg/`)**: 
  - `detection`: Identifies 11+ game engines automatically (Unity, Unreal, UE3, Godot, SAGE, GoldSrc, Source 2, FromSoftware, Factorio, Zomboid, RAGE) with a nested-dir resolver for games installed one or two levels deep.
  - `extract`: Parses game assets via embedded Python scripts (Unity), native Go parsers (Valve KeyValues, Lua tables, Factorio cfg, SAGE `.big/.csf`), and CLI tools (repak, gdre_tools, fftools).
  - `inject`: Applies Persian shaping and re-injects translated strings back into `.assets` / `.locres` / `.pck` files.
  - `tools`: A smart registry that dynamically resolves required external dependencies (UnityPy, gdre_tools, repak, fftools).
- **Frontend (`frontend/`)**: Modern UI powered by TailwindCSS and Framer Motion.
- **Patcher (`patcher/`)**: Standalone C# GUI patcher (`FarsiForgePatcher.exe`, FFP1 format) that installs a translation patch directly onto the game.

---

## 🚀 Getting Started

### Prerequisites
- [Go 1.21+](https://golang.org/dl/)
- [Node.js 18+](https://nodejs.org/)
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
4. **Inject**: FarsiForge applies RTL reshaping to your Persian text and repacks the assets safely.
5. **Patch**: Build a final standalone patcher for your community.

---

## 🤝 Contributing
Contributions, issues, and feature requests are welcome! 
Feel free to check the [issues page](../../issues).

## 📄 License
This project is licensed under the [MIT License](LICENSE).

<div align="center">
  <i>Developed with ❤️ for the Persian Gaming Community</i>
</div>
