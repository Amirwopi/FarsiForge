package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"farsiforge/pkg/detection"
	"farsiforge/pkg/project"
	"farsiforge/pkg/tools"
)

// RPGMakerExtractor extracts strings from RPG Maker MV/MZ games.
// MV/MZ use JSON files in www/data/ for all game text.
type RPGMakerExtractor struct{}

func (e *RPGMakerExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction:     true,
		DialogueExtraction: true,
	}
}

func (e *RPGMakerExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	dataDir := info.DataPath
	if dataDir == "" {
		// Try to find www/ or data/
		dataDir = filepath.Join(info.GameRoot, "www", "data")
		if !dirExists(dataDir) {
			dataDir = filepath.Join(info.GameRoot, "data")
		}
	}
	if !dirExists(dataDir) {
		return fmt.Errorf("RPG Maker data directory not found: %s", dataDir)
	}

	// Known RPG Maker MV/MZ data files and their text fields
	type fieldSpec struct {
		fields   []string
		context  string
		nestedObj string // if non-empty, extract from array of objects with this key
		nestedFields []string
	}

	fileSpecs := map[string]fieldSpec{
		"Actors.json":     {fields: []string{"name", "nickname", "profile"}, context: "actor"},
		"Classes.json":    {fields: []string{"name"}, context: "class"},
		"Skills.json":     {fields: []string{"name", "description", "message1", "message2"}, context: "skill"},
		"Items.json":      {fields: []string{"name", "description"}, context: "item"},
		"Weapons.json":    {fields: []string{"name", "description"}, context: "weapon"},
		"Armors.json":     {fields: []string{"name", "description"}, context: "armor"},
		"Enemies.json":    {fields: []string{"name"}, context: "enemy"},
		"Troops.json":     {context: "troop", nestedObj: "pages", nestedFields: []string{"list"}},
		"States.json":     {fields: []string{"name", "description", "message1", "message2", "message3", "message4"}, context: "state"},
		"Animations.json": {fields: []string{"name"}, context: "animation"},
		"Tilesets.json":   {fields: []string{"name"}, context: "tileset"},
		"CommonEvents.json": {context: "common_event", nestedObj: "list", nestedFields: []string{"code", "parameters"}},
		"System.json":     {fields: []string{"gameTitle"}, context: "system"},
		"Terms.json":      {context: "terms"},
		"MapInfos.json":   {fields: []string{"name"}, context: "map_info"},
	}

	// Process each known data file
	for fileName, spec := range fileSpecs {
		path := filepath.Join(dataDir, fileName)
		if !fileExists(path) {
			continue
		}
		extractRPGMakerFile(path, fileName, spec, proj, info.GameRoot)
	}

	// Process map files (Map001.json, Map002.json, etc.)
	entries, err := os.ReadDir(dataDir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, "Map") && strings.HasSuffix(strings.ToLower(name), ".json") {
				path := filepath.Join(dataDir, name)
				extractRPGMakerMap(path, name, proj, info.GameRoot)
			}
		}
	}

	return nil
}

// extractRPGMakerFile processes a standard RPG Maker data JSON file.
func extractRPGMakerFile(path, fileName string, spec struct {
	fields       []string
	context      string
	nestedObj    string
	nestedFields []string
}, proj *project.Project, gameRoot string) {

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	relFile, _ := filepath.Rel(gameRoot, path)

	// RPG Maker JSON files are typically arrays of objects (with null at index 0)
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// Try as object
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			return
		}
		for key, val := range obj {
			if s, ok := val.(string); ok && len(s) > 0 {
				proj.AddEntry(project.StringEntry{
					Source:  s,
					File:    relFile,
					Path:    fmt.Sprintf("%s.%s", fileName, key),
					Context: spec.context,
					Status:  project.StatusUntranslated,
				})
			}
		}
		return
	}

	for i, item := range raw {
		if string(item) == "null" {
			continue
		}

		var obj map[string]interface{}
		if err := json.Unmarshal(item, &obj); err != nil {
			continue
		}

		// Extract direct fields
		for _, field := range spec.fields {
			if val, ok := obj[field]; ok {
				if s, ok := val.(string); ok && len(s) > 0 {
					proj.AddEntry(project.StringEntry{
						Source:  s,
						File:    relFile,
						Path:    fmt.Sprintf("%s[%d].%s", fileName, i, field),
						Context: spec.context,
						Status:  project.StatusUntranslated,
					})
				}
			}
		}

		// Handle nested objects (e.g. troops.pages, common_events.list)
		if spec.nestedObj != "" {
			if nested, ok := obj[spec.nestedObj]; ok {
				switch arr := nested.(type) {
				case []interface{}:
					for j, item := range arr {
						if m, ok := item.(map[string]interface{}); ok {
							extractRPGMakerNested(m, spec.nestedFields, relFile,
								fmt.Sprintf("%s[%d].%s[%d]", fileName, i, spec.nestedObj, j),
								spec.context, proj)
						}
					}
				}
			}
		}
	}
}

// extractRPGMakerNested extracts text from nested structures (event commands).
func extractRPGMakerNested(obj map[string]interface{}, fields []string, file, pathPrefix, context string, proj *project.Project) {
	code, _ := obj["code"].(float64)
	params, _ := obj["parameters"].([]interface{})

	// RPG Maker event command codes:
	// 101: Show Text (parameters: [actorName, faceImage, faceIndex, background, position])
	// 401: Text Line (parameters: [text])
	// 102: Show Choices (parameters: [[choice1, choice2, ...], ...])
	// 402: When Choice (parameters: [index, text])
	// 355: Script
	// 655: Script line

	switch int(code) {
	case 401: // Text line
		if len(params) > 0 {
			if s, ok := params[0].(string); ok && len(s) > 0 {
				proj.AddEntry(project.StringEntry{
					Source:  s,
					File:    file,
					Path:    fmt.Sprintf("%s.text", pathPrefix),
					Context: "dialogue",
					Status:  project.StatusUntranslated,
				})
			}
		}
	case 101: // Show Text header (character name)
		if len(params) > 0 {
			if s, ok := params[0].(string); ok && len(s) > 0 && s != "" {
				proj.AddEntry(project.StringEntry{
					Source:  s,
					File:    file,
					Path:    fmt.Sprintf("%s.actor", pathPrefix),
					Context: "character_name",
					Status:  project.StatusUntranslated,
				})
			}
		}
	case 102: // Show Choices
		if len(params) > 0 {
			if choices, ok := params[0].([]interface{}); ok {
				for k, choice := range choices {
					if s, ok := choice.(string); ok && len(s) > 0 {
						proj.AddEntry(project.StringEntry{
							Source:  s,
							File:    file,
							Path:    fmt.Sprintf("%s.choice[%d]", pathPrefix, k),
							Context: "choice",
							Status:  project.StatusUntranslated,
						})
					}
				}
			}
		}
	case 402: // When Choice
		if len(params) > 1 {
			if s, ok := params[1].(string); ok && len(s) > 0 {
				proj.AddEntry(project.StringEntry{
					Source:  s,
					File:    file,
					Path:    fmt.Sprintf("%s.choice_text", pathPrefix),
					Context: "choice",
					Status:  project.StatusUntranslated,
				})
			}
		}
	}
}

// extractRPGMakerMap processes a map JSON file (contains events with dialogue).
func extractRPGMakerMap(path, fileName string, proj *project.Project, gameRoot string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	relFile, _ := filepath.Rel(gameRoot, path)

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}

	// Map display name
	if name, ok := obj["displayName"].(string); ok && len(name) > 0 {
		proj.AddEntry(project.StringEntry{
			Source:  name,
			File:    relFile,
			Path:    "displayName",
			Context: "map_name",
			Status:  project.StatusUntranslated,
		})
	}

	// Events contain pages with event commands
	events, _ := obj["events"].([]interface{})
	for i, ev := range events {
		if ev == nil {
			continue
		}
		evObj, ok := ev.(map[string]interface{})
		if !ok {
			continue
		}

		// Event name
		if name, ok := evObj["name"].(string); ok && len(name) > 0 {
			proj.AddEntry(project.StringEntry{
				Source:  name,
				File:    relFile,
				Path:    fmt.Sprintf("events[%d].name", i),
				Context: "event_name",
				Status:  project.StatusUntranslated,
			})
		}

		// Pages
		pages, _ := evObj["pages"].([]interface{})
		for j, page := range pages {
			pageObj, ok := page.(map[string]interface{})
			if !ok {
				continue
			}
			list, _ := pageObj["list"].([]interface{})
			for k, cmd := range list {
				cmdObj, ok := cmd.(map[string]interface{})
				if !ok {
					continue
				}
				pathPrefix := fmt.Sprintf("events[%d].pages[%d].list[%d]", i, j, k)
				extractRPGMakerNested(cmdObj, nil, relFile, pathPrefix, "map_event", proj)
			}
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
