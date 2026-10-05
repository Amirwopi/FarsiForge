# FarsiForge (فارسی‌فورج) 🚀

FarsiForge is an advanced, production-ready localization and patching framework designed specifically to bring Persian (Farsi) support to video games. It automates the complex pipeline of detecting game engines, extracting text assets, fixing RTL (Right-to-Left) text shaping, and safely injecting translations back into the game.

## 🌟 Features

- **Automated Engine Detection**: Dynamically scans game directories to detect engines (Unity, Unreal, Godot) with a high-confidence scoring system.
- **Plugin-Based Architecture**: Extractor and Injector logic is strictly decoupled. Adding support for a new engine (e.g., RPGMaker, RenPy) is as simple as dropping in a new plugin.
- **Robust Safety Mechanisms**: Employs SHA-256 hashing and sidecar backups. It never modifies original game files without a secure rollback point.
- **Advanced Persian Text Processing**: Built-in logic for:
  - RTL (Right-to-Left) BiDi reordering.
  - Arabic/Persian Glyph Reshaping (contextual forms).
  - Arabic Yeh/Kaf to Persian Yeh/Kaf normalization.
  - Persian digit conversion.
- **Professional UI/UX**: Includes a high-end Glassmorphism Cyberpunk React/Next.js dashboard for seamless visual interaction.
- **Headless CLI**: Includes a lightning-fast CLI for automated pipelines and debugging.

---

## 🛠️ Getting Started

### 1. Requirements
- **Go 1.21+** (for building the core engine)
- **Node.js 20+** (for the UI dashboard)
- **External Tools**: UnityPy, UnrealLocres, gdre_tools (Place inside `Tools/` directory if needed for specific engines).

### 2. Building the Project

Compile the Go binaries to the `bin/` folder:
```powershell
mkdir bin
go build -o bin/farsiforge-api.exe ./cmd/farsiforge
go build -o bin/farsiforge-cli.exe ./cmd/farsiforge-cli
```

Build the Frontend UI:
```powershell
cd frontend
npm install
npm run build
```

---

## 💻 Usage

### Option 1: Modern Web Dashboard (Recommended)
Launch the API and the Next.js frontend:
```powershell
# 1. Start the API
.\bin\farsiforge-api.exe

# 2. Start the UI
cd frontend
npm run dev
```
Open `http://localhost:3000` to access the FarsiForge Wizard.

### Option 2: Command Line Interface (CLI)
For developers or automated environments, use the CLI to run the full pipeline safely:
```powershell
.\bin\farsiforge-cli.exe -dir "D:\Path\To\Game" -action pipeline
```

---

## 🏗️ Architecture
- `pkg/core`: Core data models and plugin interfaces (`IExtractor`, `IInjector`).
- `pkg/detection`: Registry-based engine signature scanner.
- `pkg/extract` & `pkg/inject`: Implementation of extraction/injection logic per engine.
- `pkg/persian`: Standardized Persian string manipulation.
- `pkg/backup`: Transactional backup manager with SHA256 integrity.

## 🤝 Contributing
FarsiForge uses a strict Registry Pattern. To add a new engine, implement the `core.IExtractor` and `core.IInjector` interfaces and register them in their respective packages. No `switch/case` hardcoding is allowed!
