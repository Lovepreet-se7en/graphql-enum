package exporter

import (
	"encoding/json"
	"os"
	"time"

	"github.com/lovepreet-se7en/graphql-enum/internal/schema"
)

type PathExport struct {
	Index     int      `json:"index"`
	Path      string   `json:"path"`
	Segments  []string `json:"segments"`
	Depth     int      `json:"depth"`
	Query     string   `json:"query,omitempty"`
	Arguments []ArgExport `json:"arguments,omitempty"`
}

type ArgExport struct {
	Field    string `json:"field"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type ExportData struct {
	GeneratedAt string       `json:"generated_at"`
	TargetType  string       `json:"target_type"`
	SchemaFile  string       `json:"schema_file"`
	TotalPaths  int          `json:"total_paths"`
	EntryPoints []string     `json:"entry_points"`
	Paths       []PathExport `json:"paths"`
}

func ToJSON(paths []schema.GraphQLPath, targetType, schemaFile string) ([]byte, error) {
	export := ExportData{
		GeneratedAt: time.Now().Format(time.RFC3339),
		TargetType:  targetType,
		SchemaFile:  schemaFile,
		TotalPaths:  len(paths),
		Paths:       make([]PathExport, len(paths)),
	}

	entryPoints := make(map[string]bool)
	
	for i, path := range paths {
		segments := make([]string, len(path.Segments))
		for j, seg := range path.Segments {
			segments[j] = seg.Name
			if j == 0 {
				entryPoints[seg.Name] = true
			}
		}

		pathStr := formatPath(path)
		
		// Collect arguments
		var args []ArgExport
		for _, seg := range path.Segments {
			for _, arg := range seg.Args {
				args = append(args, ArgExport{
					Field:    seg.Name,
					Name:     arg.Name,
					Type:     arg.Type,
					Required: arg.Required,
				})
			}
		}

		export.Paths[i] = PathExport{
			Index:     i + 1,
			Path:      pathStr,
			Segments:  segments,
			Depth:     path.Depth,
			Arguments: args,
		}
	}

	// Convert entry points map to slice
	for ep := range entryPoints {
		export.EntryPoints = append(export.EntryPoints, ep)
	}

	return json.MarshalIndent(export, "", "  ")
}

func SaveToFile(paths []schema.GraphQLPath, targetType, schemaFile, filename string) error {
	data, err := ToJSON(paths, targetType, schemaFile)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func formatPath(path schema.GraphQLPath) string {
	var parts []string
	for _, seg := range path.Segments {
		parts = append(parts, seg.Name)
	}
	// Simple join with arrows for readability
	return joinParts(parts, " → ")
}

func joinParts(parts []string, separator string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, part := range parts[1:] {
		result += separator + part
	}
	return result
}
