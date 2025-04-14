package templates

import (
	"bytes"
	"html/template"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/temirov/RSVP/pkg/config"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

var goldmarkMarkdownRenderer = goldmark.New(
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
	),
)

var customTemplateFunctions = template.FuncMap{
	"currentYear": func() int { return time.Now().Year() },
	"renderMarkdown": func(inputText string) template.HTML {
		var outputBuffer bytes.Buffer
		conversionError := goldmarkMarkdownRenderer.Convert([]byte(inputText), &outputBuffer)
		if conversionError != nil {
			return ""
		}
		return template.HTML(outputBuffer.String())
	},
}

var PrecompiledTemplatesMap map[string]*template.Template

// LoadAllPrecompiledTemplates discovers and parses all application templates integrated with layout.
func LoadAllPrecompiledTemplates(templatesDirectoryPath string) {
	PrecompiledTemplatesMap = make(map[string]*template.Template)
	mainViewTemplateNames := []string{
		config.TemplateEvents,
		config.TemplateRSVPs,
		config.TemplateRSVP,
		config.TemplateResponse,
		config.TemplateThankYou,
		config.TemplateVenues,
	}
	var layoutFilePath string
	var partialTemplateFiles []string
	mainViewFilePaths := make(map[string]string)
	directoryWalkError := filepath.WalkDir(templatesDirectoryPath, func(filePath string, directoryEntry fs.DirEntry, walkError error) error {
		if walkError != nil {
			log.Printf("Warning: Error accessing path %q during template walk: %v", filePath, walkError)
			return walkError
		}
		if directoryEntry.IsDir() {
			return nil
		}
		isTemplateFile := strings.HasSuffix(directoryEntry.Name(), config.TemplateExtension)
		if !isTemplateFile {
			return nil
		}
		baseTemplateName := strings.TrimSuffix(directoryEntry.Name(), config.TemplateExtension)
		relativeFilePath, _ := filepath.Rel(templatesDirectoryPath, filePath)
		if baseTemplateName == config.TemplateLanding {
			log.Printf("Skipping standalone template: %s", relativeFilePath)
			return nil
		}
		if baseTemplateName == config.TemplateLayout {
			if layoutFilePath != "" {
				log.Printf("Multiple layout files found; using '%s' and ignoring '%s'", layoutFilePath, filePath)
			} else {
				layoutFilePath = filePath
				log.Printf("Found layout: %s", relativeFilePath)
			}
		} else if strings.HasPrefix(relativeFilePath, config.PartialsDir+string(filepath.Separator)) || strings.HasPrefix(baseTemplateName, "_") {
			partialTemplateFiles = append(partialTemplateFiles, filePath)
			log.Printf("Found partial: %s", relativeFilePath)
		} else {
			mainViewFound := false
			for _, mainViewName := range mainViewTemplateNames {
				if baseTemplateName == mainViewName {
					if existingFilePath, fileFound := mainViewFilePaths[mainViewName]; fileFound {
						log.Printf("Multiple files found for main view '%s'; using '%s' and ignoring '%s'", mainViewName, existingFilePath, filePath)
					} else {
						mainViewFilePaths[mainViewName] = filePath
						mainViewFound = true
						log.Printf("Found main view: %s (for %s)", relativeFilePath, mainViewName)
					}
					break
				}
			}
			if !mainViewFound {
				log.Printf("Found template file '%s' not identified as layout, partial, or known main view. Ignoring in layout system.", relativeFilePath)
			}
		}
		return nil
	})
	if directoryWalkError != nil {
		log.Fatalf("FATAL: Error walking template directory '%s': %v", templatesDirectoryPath, directoryWalkError)
	}
	if layoutFilePath == "" {
		log.Fatalf("FATAL: Layout file '%s%s' not found in directory '%s'", config.TemplateLayout, config.TemplateExtension, templatesDirectoryPath)
	}
	log.Printf("Found %d partial template files.", len(partialTemplateFiles))
	parsedTemplateCount := 0
	for _, mainViewName := range mainViewTemplateNames {
		log.Printf("Parsing template set for entry point: %s", mainViewName)
		mainViewFilePath, fileFound := mainViewFilePaths[mainViewName]
		if !fileFound {
			log.Printf("Main view file for '%s' not found. Skipping parsing for this view.", mainViewName)
			continue
		}
		filesForTemplateSet := []string{layoutFilePath}
		filesForTemplateSet = append(filesForTemplateSet, partialTemplateFiles...)
		filesForTemplateSet = append(filesForTemplateSet, mainViewFilePath)
		templateSet, parseError := template.New(filepath.Base(mainViewFilePath)).
			Funcs(customTemplateFunctions).
			ParseFiles(filesForTemplateSet...)
		if parseError != nil {
			log.Fatalf("FATAL: Failed to parse template set for view '%s'. Error: %v. Files: %v", mainViewName, parseError, filesForTemplateSet)
		}
		if templateSet.Lookup(config.TemplateLayout) == nil {
			log.Fatalf("FATAL: Template set for '%s' parsed, but '%s' template definition missing.", mainViewName, config.TemplateLayout)
		}
		PrecompiledTemplatesMap[mainViewName] = templateSet
		log.Printf("Successfully parsed and cached template set for: %s", mainViewName)
		parsedTemplateCount++
	}
	if parsedTemplateCount != len(mainViewTemplateNames) {
		log.Printf("Processed %d template sets, but expected %d based on main view names list.", parsedTemplateCount, len(mainViewTemplateNames))
	}
	log.Println("Layout-integrated template loading complete.")
}
