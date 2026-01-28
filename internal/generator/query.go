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
				// Always include arguments with variables to make them configurable
				argParts = append(argParts, fmt.Sprintf("%s: $%s", arg.Name, varName))
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
		if f.Type != typeName { // Avoid recursion
			b.WriteString("\n" + indent + f.Name)

			// Check if this field is a connection type that needs subselections
			if g.isConnectionType(f.Type) {
				b.WriteString(" {")
				// Add basic fields for connection types
				b.WriteString("\n" + indent + "  __typename")

				// If it's a connection, try to get the node type and add some fields
				nodeType := g.getNodeType(f.Type)
				if nodeType != "" {
					nodeTyp := g.schema.GetType(nodeType)
					if nodeTyp != nil {
						// Add a few basic fields from the node type
						fieldCount := 0
						for _, nodeField := range nodeTyp.Fields {
							if len(nodeField.Args) == 0 && nodeField.Name != "__typename" {
								b.WriteString("\n" + indent + "  " + nodeField.Name)
								fieldCount++
								if fieldCount >= 2 { // Limit to 2 fields to avoid overly complex queries
									break
								}
							}
						}
					}
				}
				b.WriteString("\n" + indent + "}")
			}

			count++
			if count >= 3 {
				break
			}
		}
	}
}

// isConnectionType checks if a type is a connection type (ends with Connection)
func (g *Generator) isConnectionType(typeName string) bool {
	// Remove non-null (!) and list ([...]) markers to check the base type
	baseType := g.getBaseTypeName(typeName)
	return strings.HasSuffix(baseType, "Connection")
}

// getNodeType attempts to extract the node type from a connection type
func (g *Generator) getNodeType(connectionType string) string {
	// Remove non-null (!) and list ([...]) markers
	baseType := g.getBaseTypeName(connectionType)

	// If it's a Connection type, try to find the corresponding node type
	// Usually connections have an 'edges' field that leads to a node
	connType := g.schema.GetType(baseType)
	if connType != nil {
		// Look for edges field which typically contains the nodes
		for _, field := range connType.Fields {
			if field.Name == "edges" {
				edgeType := g.getBaseTypeName(field.Type)
				edgeTyp := g.schema.GetType(edgeType)
				if edgeTyp != nil {
					// Look for 'node' field in the edge type
					for _, edgeField := range edgeTyp.Fields {
						if edgeField.Name == "node" {
							return g.getBaseTypeName(edgeField.Type)
						}
					}
				}
			}
		}
	}
	return ""
}

// getBaseTypeName removes GraphQL type modifiers like ! and []
func (g *Generator) getBaseTypeName(typeName string) string {
	// Remove non-null marker
	result := strings.TrimSuffix(typeName, "!")

	// Remove list markers [Type] -> Type
	if strings.HasPrefix(result, "[") && strings.HasSuffix(result, "]") {
		// Extract content between brackets, removing potential non-null markers
		result = strings.Trim(result, "[]!")
	}

	return result
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
