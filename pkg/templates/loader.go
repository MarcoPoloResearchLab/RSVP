// Package templates handles the loading and management of HTML templates integrated with the layout.
package templates

import (
	"html/template"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/temirov/RSVP/pkg/config"
)

var customFunctions = template.FuncMap{"currentYear": func() int { return time.Now().Year() }}

// PrecompiledTemplatesMap stores precompiled template sets (layout + partials + main view),
// keyed by the main view's base name. Standalone templates are not stored here.
var PrecompiledTemplatesMap map[string]*template.Template

// LoadAllPrecompiledTemplates walks the specified directory, identifies layout, partials,
// and main view templates (excluding standalone ones like landing.tmpl), and parses them into distinct template sets.
func LoadAllPrecompiledTemplates(templatesDirectoryPath string) {
	PrecompiledTemplatesMap = make(map[string]*template.Template)

	// Main views that integrate with the layout system
	mainViewTemplateBaseNames := []string{
		config.TemplateEvents,
		config.TemplateRSVPs,
		config.TemplateRSVP,
		config.TemplateResponse,
		config.TemplateThankYou,
	}

	log.Println("Loading application templates integrated with layout...")

	var layoutFile string
	var partialFiles []string
	var mainViewFiles = make(map[string]string)

	err := filepath.WalkDir(templatesDirectoryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		isTmplFile := strings.HasSuffix(d.Name(), config.TemplateExtension)
		if !isTmplFile {
			return nil
		}

		baseName := strings.TrimSuffix(d.Name(), config.TemplateExtension)
		relativePath, _ := filepath.Rel(templatesDirectoryPath, path)

		// Explicitly skip the standalone landing page from this loader
		if d.Name() == "landing"+config.TemplateExtension {
			log.Printf("  Skipping standalone template: %s", relativePath)
			return nil
		}

		if d.Name() == "layout"+config.TemplateExtension {
			layoutFile = path
			log.Printf("  Found layout: %s", relativePath)
		} else if strings.HasPrefix(relativePath, "partials"+string(filepath.Separator)) || strings.HasPrefix(d.Name(), "_") {
			partialFiles = append(partialFiles, path)
			log.Printf("  Found partial: %s", relativePath)
		} else {
			isMainView := false
			for _, mainName := range mainViewTemplateBaseNames {
				if baseName == mainName {
					mainViewFiles[mainName] = path
					isMainView = true
					log.Printf("  Found main view: %s (for %s)", relativePath, mainName)
					break
				}
			}
			if !isMainView {
				log.Printf("  WARN: Found template file '%s' not identified as layout, partial, or known main view. Ignoring.", relativePath)
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("FATAL: Error walking template directory '%s': %v", templatesDirectoryPath, err)
	}
	if layoutFile == "" {
		log.Fatalf("FATAL: Layout file 'layout.tmpl' not found in directory '%s'", templatesDirectoryPath)
	}
	log.Printf("Found %d partial files.", len(partialFiles))

	for _, mainViewName := range mainViewTemplateBaseNames {
		log.Printf("Parsing template set for entry point: %s", mainViewName)
		mainViewFilePath, ok := mainViewFiles[mainViewName]
		if !ok {
			log.Printf("WARN: Main view file for '%s' not found. Skipping.", mainViewName)
			continue
		}

		filesForThisSet := []string{layoutFile}
		filesForThisSet = append(filesForThisSet, partialFiles...)
		filesForThisSet = append(filesForThisSet, mainViewFilePath)

		ts, parseErr := template.New(filepath.Base(mainViewFilePath)).Funcs(customFunctions).ParseFiles(filesForThisSet...)
		if parseErr != nil {
			log.Fatalf("FATAL: Failed to parse template set for view '%s'. Error: %v", mainViewName, parseErr)
		}
		if ts.Lookup("layout") == nil {
			log.Fatalf("FATAL: Template set for '%s' parsed, but 'layout' template missing.", mainViewName)
		}
		PrecompiledTemplatesMap[mainViewName] = ts
		log.Printf("Successfully parsed template set for: %s", mainViewName)
	}
	if len(PrecompiledTemplatesMap) != len(mainViewTemplateBaseNames) {
		log.Printf("WARN: Some expected main views were not found or failed to parse.")
	}
	log.Println("Layout-integrated template loading complete.")
}
