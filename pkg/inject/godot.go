package inject

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/tools"
)

type godotTranslationMessage struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Source      string `json:"source"`
	Translation string `json:"translation"`
	Locale      string `json:"locale"`
}

func injectGodotTranslations(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	if info == nil || proj == nil || reg == nil {
		return nil, core.NewError("inject", "game, project, and tool registry are required")
	}
	fftools := fftoolsPath(reg)
	if fftools == "" || !reg.IsAvailable("fftools") {
		return nil, core.ErrToolNotFound("fftools")
	}
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return nil, err
	}

	type targetResource struct {
		container string
		file      string
		entries   []core.StringEntry
	}
	resources := make(map[string]*targetResource)
	for _, entry := range proj.FindTranslated() {
		if !strings.EqualFold(filepath.Ext(entry.File), ".translation") || !isPersianGodotLocale(godotContextLocale(entry.Context)) {
			continue
		}
		container, err := cleanGameRelativePath(entry.Container)
		if err != nil || entry.Container == "" {
			return nil, fmt.Errorf("Godot entry %q has no valid source PCK container", entry.ID)
		}
		resource, err := cleanGameRelativePath(entry.File)
		if err != nil {
			return nil, fmt.Errorf("Godot entry %q has invalid translation resource path: %w", entry.ID, err)
		}
		key := strings.ToLower(filepath.ToSlash(container)) + "\x00" + strings.ToLower(filepath.ToSlash(resource))
		if resources[key] == nil {
			resources[key] = &targetResource{container: filepath.ToSlash(container), file: filepath.ToSlash(resource)}
		}
		resources[key].entries = append(resources[key].entries, entry)
	}
	if len(resources) == 0 {
		return nil, core.NewError("inject", "no translated entries target an existing Persian Godot .translation resource; scenes/scripts and adding a new locale resource are not supported")
	}

	byContainer := make(map[string][]*targetResource)
	for _, resource := range resources {
		byContainer[resource.container] = append(byContainer[resource.container], resource)
	}
	containers := make([]string, 0, len(byContainer))
	for container := range byContainer {
		containers = append(containers, container)
	}
	sort.Strings(containers)

	tempRoot, err := os.MkdirTemp(workDir, "godot-inject-")
	if err != nil {
		return nil, fmt.Errorf("create Godot injection workspace: %w", err)
	}
	defer os.RemoveAll(tempRoot)
	result := &core.InjectionResult{}

	for packIndex, container := range containers {
		packPath, err := gameFilePath(info.GameRoot, container)
		if err != nil {
			return nil, err
		}
		if !fileExists(packPath) {
			return nil, fmt.Errorf("Godot source PCK is missing: %s", container)
		}
		if !strings.EqualFold(filepath.Ext(container), ".pck") {
			return nil, fmt.Errorf("Godot source container is not a PCK: %s", container)
		}
		packWork := filepath.Join(tempRoot, fmt.Sprintf("pack-%04d", packIndex))
		extractedDir := filepath.Join(packWork, "files")
		if err := os.MkdirAll(packWork, 0o755); err != nil {
			return nil, err
		}
		if out, err := tools.RunSilent(ctx, workDir, fftools, "pck", "extract", packPath, "-o", extractedDir); err != nil {
			return nil, core.NewError("inject", fmt.Sprintf("cannot extract PCK %s: %s", container, firstLines(out, 5)))
		}

		for _, resource := range byContainer[container] {
			relResource, err := cleanGameRelativePath(resource.file)
			if err != nil {
				return nil, err
			}
			resourcePath := filepath.Join(extractedDir, relResource)
			if !fileExists(resourcePath) {
				return nil, fmt.Errorf("Godot translation resource was not present in %s: %s", container, resource.file)
			}
			catalog := findGodotSourceCatalog(resourcePath, extractedDir)
			if catalog == "" {
				return nil, fmt.Errorf("no unambiguous source CSV catalog for %s in %s", resource.file, container)
			}
			sourceLanguage, err := godotSourceLanguage(catalog)
			if err != nil {
				return nil, fmt.Errorf("read Godot source catalog %s: %w", catalog, err)
			}
			messagesPath := filepath.Join(packWork, fmt.Sprintf("messages-%04d.json", result.StringCount))
			out, err := tools.RunSilent(ctx, workDir, fftools, "translation", "export", resourcePath, catalog, "--source-language", sourceLanguage, "-o", messagesPath)
			if err != nil {
				return nil, core.NewError("inject", fmt.Sprintf("cannot map source keys for %s: %s", resource.file, firstLines(out, 5)))
			}
			encoded, err := os.ReadFile(messagesPath)
			if err != nil {
				return nil, fmt.Errorf("read exported Godot messages: %w", err)
			}
			var messages []godotTranslationMessage
			if err := json.Unmarshal(encoded, &messages); err != nil {
				return nil, fmt.Errorf("parse exported Godot messages: %w", err)
			}
			entryPrefix := "godot:" + container + "::" + resource.file + "::"
			byID := make(map[string]core.StringEntry, len(resource.entries))
			for _, entry := range resource.entries {
				if !strings.HasPrefix(entry.ID, entryPrefix) {
					return nil, fmt.Errorf("Godot entry ID does not match its recorded PCK/resource path: %s", entry.ID)
				}
				messageID := strings.TrimPrefix(entry.ID, entryPrefix)
				if _, exists := byID[messageID]; exists {
					return nil, fmt.Errorf("duplicate translated Godot message ID %s", messageID)
				}
				byID[messageID] = entry
			}
			matched := make(map[string]bool, len(byID))
			for i := range messages {
				entry, exists := byID[messages[i].ID]
				if !exists {
					continue
				}
				if entry.Path != messages[i].Key || entry.Source != messages[i].Source || !isPersianGodotLocale(messages[i].Locale) {
					return nil, fmt.Errorf("Godot source identity changed for entry %s; extract the game again before injection", entry.ID)
				}
				messages[i].Translation = processPersianTranslation(entry.Translation, opts)
				matched[messages[i].ID] = true
				result.StringCount++
			}
			if len(matched) != len(byID) {
				return nil, fmt.Errorf("Godot export matched %d of %d translated messages in %s", len(matched), len(byID), resource.file)
			}
			updatedMessages, err := json.MarshalIndent(messages, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("encode translated Godot messages: %w", err)
			}
			if err := os.WriteFile(messagesPath, append(updatedMessages, '\n'), 0o600); err != nil {
				return nil, fmt.Errorf("write translated Godot message input: %w", err)
			}
			translatedResource := filepath.Join(packWork, fmt.Sprintf("translated-%04d.translation", result.StringCount))
			out, err = tools.RunSilent(ctx, workDir, fftools, "translation", "import", resourcePath, messagesPath, "-o", translatedResource)
			if err != nil {
				return nil, core.NewError("inject", fmt.Sprintf("cannot write Godot translation resource %s: %s", resource.file, firstLines(out, 5)))
			}
			resourceBytes, err := os.ReadFile(translatedResource)
			if err != nil {
				return nil, fmt.Errorf("read rewritten Godot translation: %w", err)
			}
			if err := os.WriteFile(resourcePath, resourceBytes, 0o600); err != nil {
				return nil, fmt.Errorf("stage rewritten Godot translation in extracted PCK: %w", err)
			}
		}

		rebuiltPath := filepath.Join(packWork, "rebuilt.pck")
		out, err := tools.RunSilent(ctx, workDir, fftools, "pck", "rebuild", packPath, extractedDir, "-o", rebuiltPath)
		if err != nil {
			return nil, core.NewError("inject", fmt.Sprintf("cannot rebuild Godot PCK %s: %s", container, firstLines(out, 5)))
		}
		stagedPath, err := stagedPath(workDir, "out", container)
		if err != nil {
			return nil, err
		}
		if err := copyStagedFile(rebuiltPath, stagedPath); err != nil {
			return nil, fmt.Errorf("stage rebuilt Godot PCK %s: %w", container, err)
		}
		result.ModifiedFiles = append(result.ModifiedFiles, container)
	}

	sort.Strings(result.ModifiedFiles)
	return result, nil
}

func godotContextLocale(context string) string {
	prefix := "Godot translation "
	if !strings.HasPrefix(context, prefix) {
		return ""
	}
	fields := strings.Fields(strings.TrimPrefix(context, prefix))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func isPersianGodotLocale(locale string) bool {
	locale = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(locale), "-", "_"), " ", ""))
	return locale == "fa" || strings.HasPrefix(locale, "fa_")
}

func findGodotSourceCatalog(translationPath, root string) string {
	base := strings.TrimSuffix(filepath.Base(translationPath), filepath.Ext(translationPath))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Clean(filepath.Dir(translationPath))
	var exact, sameDir []string
	if rel, err := filepath.Rel(root, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	children, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, child := range children {
		if child.IsDir() || !strings.EqualFold(filepath.Ext(child.Name()), ".csv") {
			continue
		}
		candidate := filepath.Join(dir, child.Name())
		sameDir = append(sameDir, candidate)
		if strings.EqualFold(strings.TrimSuffix(child.Name(), filepath.Ext(child.Name())), stem) {
			exact = append(exact, candidate)
		}
	}
	if len(exact) == 1 {
		return exact[0]
	}
	if len(exact) == 0 && len(sameDir) == 1 {
		return sameDir[0]
	}
	return ""
}

func godotSourceLanguage(catalog string) (string, error) {
	f, err := os.Open(catalog)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return "", err
	}
	var first string
	for _, raw := range header {
		name := strings.TrimPrefix(strings.TrimSpace(raw), "\ufeff")
		if strings.EqualFold(name, "key") || name == "" {
			continue
		}
		if strings.EqualFold(name, "EN") {
			return name, nil
		}
		if first == "" {
			first = name
		}
	}
	if first == "" {
		return "", fmt.Errorf("catalog has no language columns")
	}
	return first, nil
}

func copyStagedFile(source, destination string) (retErr error) {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	temp, err := os.CreateTemp(filepath.Dir(destination), ".ff-stage-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := io.Copy(temp, in); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, destination); err != nil {
		return err
	}
	return nil
}
