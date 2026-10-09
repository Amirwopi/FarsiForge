package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"farsiforge/pkg/detection"
	"farsiforge/pkg/exchange"
	"farsiforge/pkg/extract"
	"farsiforge/pkg/inject"
	"farsiforge/pkg/installer"

	"farsiforge/pkg/core"
	"farsiforge/pkg/tools"
)

//go:embed web
var webFS embed.FS

// Global state (single-user desktop app)
var (
	registry    *tools.Registry
	currentProj *core.Project
	currentInfo *core.GameInfo
	projectFile string
)

func main() {
	// Initialize tool registry
	var err error
	registry, err = tools.NewRegistry("D:\\FarsiForge\\Tools")
	if err != nil {
		log.Printf("Warning: %v", err)
	}

	// Find available port
	port := findAvailablePort(7842)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Setup routes
	mux := http.NewServeMux()

	// Serve embedded web UI
	webContent, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	// API routes
	mux.HandleFunc("/api/tools", handleTools)
	mux.HandleFunc("/api/detect", handleDetect)
	mux.HandleFunc("/api/extract", handleExtract)
	mux.HandleFunc("/api/project", handleProject)
	mux.HandleFunc("/api/translate", handleTranslate)
	mux.HandleFunc("/api/export", handleExport)
	mux.HandleFunc("/api/import", handleImport)
	mux.HandleFunc("/api/inject", handleInject)
	mux.HandleFunc("/api/build-installer", handleBuildInstaller)

	// Start server
	go func() {
		log.Printf("FarsiForge starting on http://%s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatal(err)
		}
	}()

	// Open browser
	url := fmt.Sprintf("http://%s", addr)
	openBrowser(url)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║  🔨 FarsiForge — فارسی‌ساز بازی              ║")
	fmt.Println("╠══════════════════════════════════════════════╣")
	fmt.Printf("║  آدرس: http://%-31s║\n", addr)
	fmt.Println("║  برای خروج: Ctrl+C                           ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println()

	select {} // block forever
}

// ─── API Handlers ──────────────────────────────────────────────────

func handleTools(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, registry)
}

func handleDetect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request")
		return
	}

	res, err := detection.DefaultRegistry().Detect(req.Path)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	dataPath := ""
	if len(res.DataPaths) > 0 {
		dataPath = res.DataPaths[0]
	}

	currentInfo = &core.GameInfo{
		Engine:     res.Engine,
		Backend:    res.Backend,
		Version:    res.Version,
		GameName:   res.GameName,
		GameExe:    res.GameExe,
		GameRoot:   req.Path,
		DataPath:   dataPath,
		Confidence: res.Confidence,
		Evidence:   res.Evidence,
		Metadata:   res.Metadata,
	}
	respondJSON(w, currentInfo)
}

func handleExtract(w http.ResponseWriter, r *http.Request) {
	var info core.GameInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		respondError(w, 400, "invalid request")
		return
	}

	currentInfo = &info

	// Create project
	gameName := info.GameName
	if gameName == "" {
		gameName = filepath.Base(info.GameRoot)
	}
	currentProj = core.NewProject(gameName+" — فارسی‌ساز", info.GameRoot, string(info.Engine))
	currentProj.GameName = gameName
	currentProj.Backend = string(info.Backend)
	currentProj.Version = info.Version

	// Run extraction
	err := extract.Run(r.Context(), &info, currentProj, registry)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	// Save project
	projectFile = filepath.Join(info.GameRoot, ".farsiforge_project.json")
	currentProj.Save(projectFile)

	stats := currentProj.Stats()
	respondJSON(w, map[string]interface{}{
		"project":     currentProj,
		"entries":     currentProj.Entries,
		"entry_count": stats.Total,
		"file_count":  len(uniqueFiles(currentProj.Entries)),
	})
}

func handleProject(w http.ResponseWriter, r *http.Request) {
	if currentProj == nil {
		respondError(w, 404, "no project loaded")
		return
	}
	respondJSON(w, currentProj)
}

func handleTranslate(w http.ResponseWriter, r *http.Request) {
	if currentProj == nil {
		respondError(w, 404, "no project loaded")
		return
	}

	var req struct {
		ID          string `json:"id"`
		Translation string `json:"translation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request")
		return
	}

	status := core.StatusTranslated
	if req.Translation == "" {
		status = core.StatusUntranslated
	}

	if err := currentProj.SetTranslation(req.ID, req.Translation, status); err != nil {
		respondError(w, 404, "entry not found")
		return
	}

	// Auto-save
	if projectFile != "" {
		currentProj.Save(projectFile)
	}

	respondJSON(w, map[string]string{"status": "ok"})
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	if currentProj == nil {
		respondError(w, 404, "no project loaded")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "xlsx"
	}

	// Create temp file
	ext := "." + format
	tmpFile := filepath.Join(os.TempDir(), "farsiforge_export"+ext)

	if err := exchange.Export(currentProj, tmpFile); err != nil {
		respondError(w, 500, err.Error())
		return
	}

	// Send file
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=farsiforge_export%s", ext))
	if format == "xlsx" {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	} else {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	}
	http.ServeFile(w, r, tmpFile)
}

func handleImport(w http.ResponseWriter, r *http.Request) {
	if currentProj == nil {
		respondError(w, 404, "no project loaded")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, 400, "no file uploaded")
		return
	}
	defer file.Close()

	// Save to temp file
	tmpFile := filepath.Join(os.TempDir(), header.Filename)
	dst, err := os.Create(tmpFile)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}
	io.Copy(dst, file)
	dst.Close()

	count, err := exchange.Import(currentProj, tmpFile)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	if projectFile != "" {
		currentProj.Save(projectFile)
	}

	respondJSON(w, map[string]interface{}{
		"imported": count,
		"entries":  currentProj.Entries,
	})
}

func handleInject(w http.ResponseWriter, r *http.Request) {
	if currentProj == nil || currentInfo == nil {
		respondError(w, 404, "no project loaded")
		return
	}

	var req struct {
		GameInfo core.GameInfo   `json:"game_info"`
		Options  map[string]bool `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, 400, "invalid request")
		return
	}

	currentInfo = &req.GameInfo

	// Build Persian processing options
	opts := core.PersianOptions{
		Reshape:       req.Options["reshape"],
		BidiReorder:   req.Options["bidi_reorder"],
		FixYeh:        req.Options["fix_yeh"],
		PersianDigits: req.Options["persian_digits"],
	}
	// Set defaults if all false
	if !opts.Reshape && !opts.BidiReorder && !opts.FixYeh && !opts.PersianDigits {
		opts = core.DefaultPersianOptions()
	}

	// Run injection
	modified, err := inject.Run(r.Context(), currentInfo, currentProj, registry, opts)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	respondJSON(w, map[string]interface{}{
		"modified_count": len(modified),
		"modified_files": modified,
	})
}

func handleBuildInstaller(w http.ResponseWriter, r *http.Request) {
	if currentProj == nil || currentInfo == nil {
		respondError(w, 404, "no project loaded")
		return
	}

	var req struct {
		PatchName   string `json:"patch_name"`
		Author      string `json:"author"`
		Description string `json:"description"`
		OutputDir   string `json:"output_dir"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.OutputDir == "" {
		req.OutputDir = filepath.Join("D:\\FarsiForge", "output", currentProj.GameName)
	}

	// Collect patch targets from the project's modified files. The patched
	// copies live in the project working directory.
	workDir := currentProj.WorkingDir
	if workDir == "" {
		workDir = filepath.Join(filepath.Dir(projectFile), "work")
	}
	var targets []installer.PatchTarget
	for _, mf := range currentProj.ModifiedFiles {
		patchedFile := filepath.Join(workDir, filepath.Base(mf))
		if !fileExistsLocal(patchedFile) {
			patchedFile = filepath.Join(currentInfo.GameRoot, mf)
		}
		if !fileExistsLocal(patchedFile) {
			continue
		}
		targets = append(targets, installer.PatchTarget{
			GamePath:    mf,
			PatchedFile: patchedFile,
		})
	}

	// Resolve the patcher exe from the tool registry.
	patcherExe := ""
	if registry != nil {
		patcherExe = filepath.Join(registry.RootDir, "patcher", "FarsiForgePatcher.exe")
	}

	cfg := installer.BuildConfig{
		GameRoot:    currentInfo.GameRoot,
		Targets:     targets,
		PatcherExe:  patcherExe,
		GameExe:     currentInfo.GameExe,
		Engine:      string(currentInfo.Engine),
		PatchName:   req.PatchName,
		Description: req.Description,
		Author:      req.Author,
		OutputDir:   req.OutputDir,
	}

	res, err := installer.Build(cfg)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}

	respondJSON(w, map[string]interface{}{
		"output_dir":   res.OutputDir,
		"patch_file":   res.PatchFile,
		"patcher_exe":  res.PatcherExe,
		"target_count": res.TargetCount,
	})
}

// fileExistsLocal is a local helper to avoid pulling in the scanner package
// just for a stat check.
func fileExistsLocal(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ─── Utilities ─────────────────────────────────────────────────────

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func findAvailablePort(start int) int {
	for port := start; port < start+100; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port
		}
	}
	return start
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func uniqueFiles(entries []core.StringEntry) []string {
	seen := make(map[string]bool)
	var files []string
	for _, e := range entries {
		if !seen[e.File] {
			seen[e.File] = true
			files = append(files, e.File)
		}
	}
	return files
}

// Ensure imports are used
var _ = strings.TrimSpace
var _ = log.Printf
