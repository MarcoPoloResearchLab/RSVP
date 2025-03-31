// Package templates handles the loading, parsing, and caching of HTML templates
// used by the RSVP application, integrating view templates with a common layout.
package templates

import (
	"html/template"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/temirov/RSVP/pkg/config" // Import config for constants
)

// customFunctions defines custom functions accessible within templates.
// Currently only includes currentYear.
var customFunctions = template.FuncMap{
	// currentYear returns the current year as an integer.
	"currentYear": func() int { return time.Now().Year() },
}

// PrecompiledTemplatesMap stores the fully parsed and ready-to-use template sets.
var PrecompiledTemplatesMap map[string]*template.Template

// LoadAllPrecompiledTemplates discovers and parses all application templates...
func LoadAllPrecompiledTemplates(templatesDirectoryPath string) {
	PrecompiledTemplatesMap = make(map[string]*template.Template)

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

	walkError := filepath.WalkDir(templatesDirectoryPath, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			log.Printf("Warning: Error accessing path %q during template walk: %v", path, walkErr)
			return walkErr
		}
		if dirEntry.IsDir() {
			return nil
		}
		isTemplateFile := strings.HasSuffix(dirEntry.Name(), config.TemplateExtension)
		if !isTemplateFile {
			return nil
		}

		baseName := strings.TrimSuffix(dirEntry.Name(), config.TemplateExtension)
		relativePath, _ := filepath.Rel(templatesDirectoryPath, path)

		if baseName == config.TemplateLanding {
			log.Printf("  Skipping standalone template: %s", relativePath)
			return nil
		}

		if baseName == config.TemplateLayout {
			if layoutFile != "" {
				log.Printf("WARN: Multiple layout files found? Using '%s', ignoring '%s'", layoutFile, path)
			} else {
				layoutFile = path
				log.Printf("  Found layout: %s", relativePath)
			}
		} else if strings.HasPrefix(relativePath, config.PartialsDir+string(filepath.Separator)) || strings.HasPrefix(baseName, "_") {
			partialFiles = append(partialFiles, path)
			log.Printf("  Found partial: %s", relativePath)
		} else {
			isMainView := false
			for _, mainViewBaseName := range mainViewTemplateBaseNames {
				if baseName == mainViewBaseName {
					if existingPath, found := mainViewFiles[mainViewBaseName]; found {
						log.Printf("WARN: Multiple files found for main view '%s'? Using '%s', ignoring '%s'", mainViewBaseName, existingPath, path)
					} else {
						mainViewFiles[mainViewBaseName] = path
						isMainView = true
						log.Printf("  Found main view: %s (for %s)", relativePath, mainViewBaseName)
					}
					break
				}
			}
			if !isMainView && baseName != config.TemplateLayout && !strings.HasPrefix(relativePath, config.PartialsDir+string(filepath.Separator)) && !strings.HasPrefix(baseName, "_") {
				log.Printf("  WARN: Found template file '%s' not identified as layout, partial, or known main view. Ignoring in layout system.", relativePath)
			}
		}
		return nil
	})

	if walkError != nil {
		log.Fatalf("FATAL: Error walking template directory '%s': %v", templatesDirectoryPath, walkError)
	}
	if layoutFile == "" {
		log.Fatalf("FATAL: Layout file '%s%s' not found in directory '%s'", config.TemplateLayout, config.TemplateExtension, templatesDirectoryPath)
	}
	log.Printf("Found %d partial template files.", len(partialFiles))

	parsedCount := 0
	for _, mainViewName := range mainViewTemplateBaseNames {
		log.Printf("Parsing template set for entry point: %s", mainViewName)
		mainViewFilePath, found := mainViewFiles[mainViewName]
		if !found {
			log.Printf("WARN: Main view file for '%s' not found. Skipping parsing for this view.", mainViewName)
			continue
		}

		filesForThisSet := []string{layoutFile}
		filesForThisSet = append(filesForThisSet, partialFiles...)
		filesForThisSet = append(filesForThisSet, mainViewFilePath)

		// Use Funcs() *before* ParseFiles()
		templateSet, parseError := template.New(filepath.Base(mainViewFilePath)).Funcs(customFunctions).ParseFiles(filesForThisSet...)
		if parseError != nil {
			log.Fatalf("FATAL: Failed to parse template set for view '%s'. Error: %v. Files: %v", mainViewName, parseError, filesForThisSet)
		}

		if templateSet.Lookup(config.TemplateLayout) == nil {
			log.Fatalf("FATAL: Template set for '%s' parsed, but '%s' template definition missing.", mainViewName, config.TemplateLayout)
		}

		PrecompiledTemplatesMap[mainViewName] = templateSet
		log.Printf("Successfully parsed and cached template set for: %s", mainViewName)
		parsedCount++
	}

	if parsedCount != len(mainViewTemplateBaseNames) {
		log.Printf("WARN: Processed %d template sets, but expected %d based on mainViewTemplateBaseNames list. Check for missing files or previous warnings.", parsedCount, len(mainViewTemplateBaseNames))
	}
	log.Println("Layout-integrated template loading complete.")
}
