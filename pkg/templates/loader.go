// File: pkg/templates/loader.go
package templates

import (
	"html/template"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"time"
)

var customFunctions = template.FuncMap{
	"currentYear": func() int {
		return time.Now().Year()
	},
	// Add other common functions needed across templates here
}

// PrecompiledTemplatesMap stores the precompiled template sets, keyed by the main view's base name.
var PrecompiledTemplatesMap map[string]*template.Template

// LoadAllPrecompiledTemplates parses templates into distinct sets for each main view.
// Each set contains the layout, all partials, and the specific main view file.
func LoadAllPrecompiledTemplates(templatesDirectoryPath string) {
	PrecompiledTemplatesMap = make(map[string]*template.Template)

	// Base names of the main view templates (e.g., "events" corresponds to "templates/events.tmpl")
	// These names are also used as keys in the PrecompiledTemplatesMap.
	mainViewTemplateBaseNames := []string{
		"events",
		"rsvps",
		"rsvp", // Corresponds to rsvp.tmpl (QR code page)
		"response",
		"thankyou",
	}

	log.Println("Loading application templates...")

	// --- 1. Find all template files and categorize them ---
	var layoutFile string
	var partialFiles []string                   // Files in partials/ or starting with _
	var mainViewFiles = make(map[string]string) // Map base name to full path

	err := filepath.WalkDir(templatesDirectoryPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tmpl") {
			baseName := strings.TrimSuffix(d.Name(), ".tmpl")
			relativePath, _ := filepath.Rel(templatesDirectoryPath, path) // Get path relative to templates dir

			if d.Name() == "layout.tmpl" {
				layoutFile = path
				log.Printf("  Found layout: %s", relativePath)
			} else if strings.HasPrefix(relativePath, "partials"+string(filepath.Separator)) || strings.HasPrefix(d.Name(), "_") {
				// Treat files in partials/ subdir OR starting with _ as partials
				partialFiles = append(partialFiles, path)
				log.Printf("  Found partial: %s", relativePath)
			} else {
				// Check if it's one of our known main views
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
					log.Printf("  WARN: Found template file '%s' not identified as layout, partial, or known main view. It will be parsed into sets but might cause conflicts if it defines common blocks.", relativePath)
					// Include unknown files as partials for now to ensure definitions are available,
					// but this might need refinement based on project structure.
					// If these unknown files define "title", "content", etc., they *will* cause issues.
					// A stricter approach might ignore them or error.
					partialFiles = append(partialFiles, path)
				}
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

	// --- 2. Create a template set for each main view ---
	for _, mainViewName := range mainViewTemplateBaseNames {
		log.Printf("Parsing template set for entry point: %s", mainViewName)

		mainViewFilePath, ok := mainViewFiles[mainViewName]
		if !ok {
			log.Printf("WARN: Main view file for '%s' not found. Skipping template set creation.", mainViewName)
			continue // Skip if the main view file itself is missing
		}

		// Files to parse for *this* specific set: layout + all partials + the one main view
		filesForThisSet := []string{}
		filesForThisSet = append(filesForThisSet, layoutFile)
		filesForThisSet = append(filesForThisSet, partialFiles...)
		filesForThisSet = append(filesForThisSet, mainViewFilePath)

		// Debug: Log files being parsed for this set
		// log.Printf("  Files for '%s' set: %v", mainViewName, filesForThisSet)

		// Create the template set. Give it a name related to the main view for clarity in errors.
		// Using filepath.Base helps ensure consistent naming regardless of OS path separators.
		ts, parseErr := template.New(filepath.Base(mainViewFilePath)). // Use the view file name as the base name for the set
										Funcs(customFunctions).
										ParseFiles(filesForThisSet...) // Parse ONLY the relevant files

		if parseErr != nil {
			// Provide more context on parsing failure
			log.Fatalf("FATAL: Failed to parse template set for view '%s'. Error parsing files %v: %v", mainViewName, filesForThisSet, parseErr)
		}

		// Sanity check: Ensure the layout was actually defined in the set
		if ts.Lookup("layout") == nil {
			log.Fatalf("FATAL: Template set for '%s' parsed successfully, but 'layout' template is missing. Check layout file path and parsing logic.", mainViewName)
		}

		PrecompiledTemplatesMap[mainViewName] = ts
		log.Printf("Successfully parsed template set for: %s", mainViewName)
	}

	// Check if any expected main views were missing
	if len(PrecompiledTemplatesMap) != len(mainViewTemplateBaseNames) {
		log.Printf("WARN: Some main views defined in 'mainViewTemplateBaseNames' were not found or failed to parse.")
	}

	log.Println("Template loading complete.")
}
