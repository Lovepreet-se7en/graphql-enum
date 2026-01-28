package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lovepreet-se7en/graphql-enum/internal/schema"
)

type Generator struct {
	schema    *schema.Schema
	outputDir string
}

type GeneratedQuery struct {
	Index       int                    `json:"index"`
	Description string                 `json:"description"`
	Path        string                 `json:"path"`
	Query       string                 `json:"query"`
	Variables   map[string]interface{} `json:"variables"`
	FileName    string                 `json:"file_name"`
}

func New(s *schema.Schema, outputDir string) *Generator {
	return &Generator{
		schema:    s,
		outputDir: outputDir,
	}
}

func (g *Generator) GenerateAll(paths []schema.GraphQLPath) ([]GeneratedQuery, error) {
	queries := make([]GeneratedQuery, len(paths))
	
	for i, path := range paths {
		queries[i] = g.generateOne(path, i+1)
	}
	
	return queries, nil
}

func (g *Generator) generateOne(path schema.GraphQLPath, index int) GeneratedQuery {
	var (
		queryBuilder strings.Builder
		vars         = make(map[string]interface{})
		varDefs      []string
	)
	
	// Collect variables
	for _, seg := range path.Segments {
		for _, arg := range seg.Args {
			varName := fmt.Sprintf("%s_%s", seg.Name, arg.Name)
			vars[varName] = g.generateExampleValue(arg.Type)
			if arg.Required {
				varDefs = append(varDefs, fmt.Sprintf("$%s: %s", varName, arg.Type))
			}
		}
	}
	
	// Build query header
	queryBuilder.WriteString("query")
	if len(varDefs) > 0 {
		queryBuilder.WriteString("(" + strings.Join(varDefs, ", ") + ")")
	}
	queryBuilder.WriteString(" {\n")
	
	// Build body
	indent := "  "
	for i, seg := range path.Segments {
		queryBuilder.WriteString(indent + seg.Name)
		
		// Add arguments
		if len(seg.Args) > 0 {
			var argParts []string
			for _, arg := range seg.Args {
				varName := fmt.Sprintf("%s_%s", seg.Name, arg.Name)
				if arg.Required {
					argParts = append(argParts, fmt.Sprintf("%s: $%s", arg.Name, varName))
				} else {
					argParts = append(argParts, fmt.Sprintf("%s: null", arg.Name))
				}
			}
			queryBuilder.WriteString("(" + strings.Join(argParts, ", ") + ")")
		}
		
		if i < len(path.Segments)-1 {
			queryBuilder.WriteString(" {\n")
			indent += "  "
		} else {
			// Last segment - add fields
			queryBuilder.WriteString(" {\n")
			g.addLeafFields(&queryBuilder, indent+"  ", seg.Type)
			queryBuilder.WriteString("\n" + indent + "}")
		}
	}
	
	// Close braces
	for i := 0; i < len(path.Segments)-1; i++ {
		indent = indent[:len(indent)-2]
		queryBuilder.WriteString("\n" + indent + "}")
	}
	
	queryBuilder.WriteString("\n}")
	
	pathStr := g.formatPath(path)
	
	return GeneratedQuery{
		Index:       index,
		Description: fmt.Sprintf("Path %d: %s", index, pathStr),
		Path:        pathStr,
		Query:       queryBuilder.String(),
		Variables:   vars,
		FileName:    fmt.Sprintf("query_%03d.graphql", index),
	}
}

func (g *Generator) addLeafFields(b *strings.Builder, indent, typeName string) {
	b.WriteString(indent + "__typename")
	
	typ := g.schema.GetType(typeName)
	if typ == nil {
		return
	}
	
	count := 0
	for _, f := range typ.Fields {
		if len(f.Args) == 0 && f.Type != typeName { // Avoid recursion
			b.WriteString("\n" + indent + f.Name)
			count++
			if count >= 3 {
				break
			}
		}
	}
}

func (g *Generator) formatPath(path schema.GraphQLPath) string {
	var parts []string
	for _, seg := range path.Segments {
		parts = append(parts, seg.Name)
	}
	return strings.Join(parts, " → ")
}

func (g *Generator) generateExampleValue(typeName string) interface{} {
	switch {
	case strings.Contains(typeName, "String"):
		return "example_string"
	case strings.Contains(typeName, "Int"):
		return 42
	case strings.Contains(typeName, "Float"):
		return 3.14
	case strings.Contains(typeName, "ID"):
		return "123"
	case strings.Contains(typeName, "Boolean"):
		return true
	case strings.HasPrefix(typeName, "["):
		return []interface{}{}
	default:
		return nil
	}
}

func (g *Generator) SaveToFiles(queries []GeneratedQuery) error {
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return err
	}
	
	// Save individual .graphql files
	for _, q := range queries {
		filename := filepath.Join(g.outputDir, q.FileName)
		content := fmt.Sprintf("# %s\n# Variables: %v\n\n%s", 
			q.Description, q.Variables, q.Query)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return err
		}
	}
	
	// Save manifest.json
	manifest := struct {
		Count   int              `json:"count"`
		Queries []GeneratedQuery `json:"queries"`
	}{
		Count:   len(queries),
		Queries: queries,
	}
	
	manifestFile := filepath.Join(g.outputDir, "manifest.json")
	data, _ := json.MarshalIndent(manifest, "", "  ")
	return os.WriteFile(manifestFile, data, 0644)
}

func (g *Generator) GenerateCurlCommands(endpoint string, queries []GeneratedQuery) []string {
	cmds := make([]string, len(queries))
	
	for i, q := range queries {
		payload := map[string]interface{}{
			"query":     q.Query,
			"variables": q.Variables,
		}
		jsonPayload, _ := json.Marshal(payload)
		
		cmds[i] = fmt.Sprintf(`# %s
curl -X POST %s \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '%s'`, q.Description, endpoint, string(jsonPayload))
	}
	
	return cmds
}
