package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// SchemaFormat represents the two supported input formats
type SchemaFormat int

const (
	FormatIntrospection SchemaFormat = iota
	FormatGitHub
)

// IntrospectionFormat represents standard GraphQL introspection JSON
type IntrospectionFormat struct {
	Data struct {
		Schema struct {
			QueryType        *NamedTypeRef `json:"queryType"`
			MutationType     *NamedTypeRef `json:"mutationType"`
			SubscriptionType *NamedTypeRef `json:"subscriptionType"`
			Types            []IntrospectionType `json:"types"`
		} `json:"__schema"`
	} `json:"data"`
}

type NamedTypeRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type IntrospectionType struct {
	Kind          string            `json:"kind"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Fields        []IntrospectionField `json:"fields"`
	Interfaces    []NamedTypeRef    `json:"interfaces"`
	PossibleTypes []NamedTypeRef    `json:"possibleTypes"`
}

type IntrospectionField struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Type        TypeRef            `json:"type"`
	Args        []IntrospectionArg `json:"args"`
}

type IntrospectionArg struct {
	Name string  `json:"name"`
	Type TypeRef `json:"type"`
}

type TypeRef struct {
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	OfType *TypeRef `json:"ofType"`
}

// GitHubFormat represents GitHub's custom schema documentation format
type GitHubFormat struct {
	Queries      []GitHubFieldDef `json:"queries"`
	Mutations    []GitHubFieldDef `json:"mutations"`
	Objects      []GitHubTypeDef  `json:"objects"`
	Interfaces   []GitHubTypeDef  `json:"interfaces"`
	Unions       []GitHubUnion    `json:"unions"`
	Enums        []GitHubEnum     `json:"enums"`
	Scalars      []GitHubScalar   `json:"scalars"`
	InputObjects []GitHubTypeDef  `json:"inputObjects"`
}

type GitHubFieldDef struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Kind         string            `json:"kind"`
	ID           string            `json:"id"`
	Href         string            `json:"href"`
	Description  string            `json:"description"`
	Args         []GitHubArg       `json:"args"`         // Top-level queries/mutations use "args"
	InputFields  []GitHubFieldDef  `json:"inputFields"`  // Mutations use inputFields
	ReturnFields []GitHubReturnField `json:"returnFields"` // Mutations have returnFields
}

type GitHubArg struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Href         string `json:"href"`
	Description  string `json:"description"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

type GitHubReturnField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	IsDeprecated bool  `json:"isDeprecated,omitempty"`
}

type GitHubTypeDef struct {
	Name        string           `json:"name"`
	Kind        string           `json:"kind"`
	ID          string           `json:"id"`
	Href        string           `json:"href"`
	Description string           `json:"description"`
	Fields      []GitHubField    `json:"fields"`
	Implements  []GitHubImplement `json:"implements"`
}

// GitHubField is used inside Objects/Interfaces and uses "arguments" (not "args")
type GitHubField struct {
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	Type              string      `json:"type"`
	ID                string      `json:"id"`
	Kind              string      `json:"kind"`
	Href              string      `json:"href"`
	Arguments         []GitHubArg `json:"arguments"` // Note: different from top-level "args"
	IsDeprecated      bool        `json:"isDeprecated,omitempty"`
	DeprecationReason string      `json:"deprecationReason,omitempty"`
}

type GitHubImplement struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	Href string `json:"href"`
}

type GitHubUnion struct {
	Name          string   `json:"name"`
	Kind          string   `json:"kind"`
	ID            string   `json:"id"`
	Href          string   `json:"href"`
	Description   string   `json:"description"`
	PossibleTypes []string `json:"possibleTypes"`
}

type GitHubEnum struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Href        string `json:"href"`
	Description string `json:"description"`
}

type GitHubScalar struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Href        string `json:"href"`
	Description string `json:"description"`
}

// Internal Graph Representation
type Graph struct {
	Nodes map[string]*Node
	Roots []string
}

type Node struct {
	Name          string
	Kind          string
	Fields        []Edge
	Implements    []string
	PossibleTypes []string
}

type Edge struct {
	Name      string
	Target    string
	Arguments []Arg
}

type Arg struct {
	Name string
	Type string
}

func main() {
	var (
		schemaFile       = flag.String("schema", "", "Path to schema JSON (introspection or GitHub format)")
		targetType       = flag.String("type", "", "Target type to find paths to (case-sensitive)")
		maxDepth         = flag.Int("max-depth", 15, "Maximum search depth")
		includeMutations = flag.Bool("mutations", false, "Include mutation paths as entry points")
		verbose          = flag.Bool("v", false, "Verbose output")
		noColor          = flag.Bool("no-color", false, "Disable colored output")
	)
	flag.Parse()

	if *noColor {
		color.NoColor = true
	}

	if *schemaFile == "" || *targetType == "" {
		printUsage()
		os.Exit(1)
	}

	// Read schema
	data, err := os.ReadFile(*schemaFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Loading schema from %s...\n", color.CyanString(*schemaFile))
	}

	// Parse schema (auto-detect format)
	graph, format, err := parseSchema(data, *includeMutations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse Error: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Detected format: %s\n", color.CyanString(format))
		fmt.Printf("Loaded %d types\n", len(graph.Nodes))
		fmt.Printf("Entry points: %s\n\n", color.CyanString(strings.Join(graph.Roots, ", ")))
	}

	// Validate target exists
	if _, exists := graph.Nodes[*targetType]; !exists {
		fmt.Fprintf(os.Stderr, "Error: Type '%s' not found in schema\n", *targetType)
		available := findSimilarTypes(graph, *targetType)
		if len(available) > 0 {
			fmt.Fprintf(os.Stderr, "Did you mean: %s?\n", strings.Join(available, ", "))
		}
		os.Exit(1)
	}

	// Find paths
	paths := findPaths(graph, *targetType, *maxDepth)

	typeNode := graph.Nodes[*targetType]
	fmt.Printf("Target: %s (%s)\n", color.CyanString(*targetType), color.YellowString(typeNode.Kind))
	fmt.Printf("Entry points: %s\n", color.CyanString(strings.Join(graph.Roots, ", ")))
	fmt.Printf("Max depth: %d\n\n", *maxDepth)

	if len(paths) == 0 {
		fmt.Printf("Warning: No paths found to %s within depth limit (%d)\n", *targetType, *maxDepth)
		os.Exit(2)
	} else {
		fmt.Printf("Found %s paths:\n\n", color.GreenString(fmt.Sprintf("%d", len(paths))))
		for i, path := range paths {
			fmt.Printf("%s %s\n", color.YellowString(fmt.Sprintf("%3d.", i+1)), formatPath(path))
		}
	}
}

func printUsage() {
	fmt.Println("GraphQL Path Enumeration Tool (Go Edition)")
	fmt.Println()
	fmt.Println("Enumerates all GraphQL paths from root queries/mutations to a target type.")
	fmt.Println("Supports standard introspection JSON and GitHub's custom schema format.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  graphql-enum -schema <file.json> -type <TypeName> [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -schema string     Path to schema JSON file (required)")
	fmt.Println("  -type string       Target type name to find paths to (required)")
	fmt.Println("  -max-depth int     Maximum traversal depth (default: 15)")
	fmt.Println("  -mutations         Include Mutation fields as entry points")
	fmt.Println("  -v                 Verbose output")
	fmt.Println("  -no-color          Disable colored output")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  graphql-enum -schema schema.json -type User")
	fmt.Println("  graphql-enum -schema github-schema.json -type Repository -mutations")
	fmt.Println("  graphql-enum -schema schema.json -type Issue -max-depth 20 -v")
}

func findSimilarTypes(graph *Graph, target string) []string {
	var suggestions []string
	targetLower := strings.ToLower(target)
	for name := range graph.Nodes {
		if strings.Contains(strings.ToLower(name), targetLower) {
			suggestions = append(suggestions, name)
		}
		if len(suggestions) >= 5 {
			break
		}
	}
	return suggestions
}

func parseSchema(data []byte, includeMutations bool) (*Graph, string, error) {
	// Try introspection format
	var intro IntrospectionFormat
	if err := json.Unmarshal(data, &intro); err == nil && intro.Data.Schema.Types != nil {
		graph := buildFromIntrospection(&intro, includeMutations)
		return graph, "GraphQL Introspection", nil
	}

	// Try GitHub format
	var gh GitHubFormat
	if err := json.Unmarshal(data, &gh); err == nil {
		if len(gh.Queries) > 0 || len(gh.Objects) > 0 || len(gh.Mutations) > 0 {
			graph := buildFromGitHubFormat(&gh, includeMutations)
			return graph, "GitHub Schema Format", nil
		}
	}

	return nil, "", fmt.Errorf("unknown schema format (expected introspection or GitHub format)")
}

func buildFromIntrospection(schema *IntrospectionFormat, includeMutations bool) *Graph {
	graph := &Graph{
		Nodes: make(map[string]*Node),
		Roots: []string{},
	}

	// Index types
	for _, t := range schema.Data.Schema.Types {
		if strings.HasPrefix(t.Name, "__") {
			continue
		}

		node := &Node{
			Name:          t.Name,
			Kind:          t.Kind,
			Fields:        []Edge{},
			Implements:    []string{},
			PossibleTypes: []string{},
		}

		for _, f := range t.Fields {
			targetType := unwrapType(f.Type)
			if targetType == "" {
				continue
			}

			edge := Edge{
				Name:   f.Name,
				Target: targetType,
			}

			for _, a := range f.Args {
				edge.Arguments = append(edge.Arguments, Arg{
					Name: a.Name,
					Type: unwrapType(a.Type),
				})
			}

			node.Fields = append(node.Fields, edge)
		}

		for _, i := range t.Interfaces {
			node.Implements = append(node.Implements, i.Name)
		}

		for _, pt := range t.PossibleTypes {
			node.PossibleTypes = append(node.PossibleTypes, pt.Name)
		}

		graph.Nodes[t.Name] = node
	}

	if schema.Data.Schema.QueryType != nil {
		graph.Roots = append(graph.Roots, schema.Data.Schema.QueryType.Name)
	}
	if includeMutations && schema.Data.Schema.MutationType != nil {
		graph.Roots = append(graph.Roots, schema.Data.Schema.MutationType.Name)
	}
	if schema.Data.Schema.SubscriptionType != nil {
		graph.Roots = append(graph.Roots, schema.Data.Schema.SubscriptionType.Name)
	}

	return graph
}

func buildFromGitHubFormat(schema *GitHubFormat, includeMutations bool) *Graph {
	graph := &Graph{
		Nodes: make(map[string]*Node),
		Roots: []string{},
	}

	// Process regular types (Objects)
	for _, obj := range schema.Objects {
		node := convertGitHubObject(obj)
		graph.Nodes[obj.Name] = node
	}

	// Process Interfaces
	for _, iface := range schema.Interfaces {
		node := convertGitHubInterface(iface)
		graph.Nodes[iface.Name] = node
	}

	// Process Unions
	for _, union := range schema.Unions {
		node := &Node{
			Name:          union.Name,
			Kind:          "UNION",
			PossibleTypes: union.PossibleTypes,
			Fields:        []Edge{},
		}
		graph.Nodes[union.Name] = node
	}

	// Process Enums and Scalars (leaf types)
	for _, enum := range schema.Enums {
		graph.Nodes[enum.Name] = &Node{Name: enum.Name, Kind: "ENUM"}
	}
	for _, scalar := range schema.Scalars {
		graph.Nodes[scalar.Name] = &Node{Name: scalar.Name, Kind: "SCALAR"}
	}

	// Build Query root from queries array
	if len(schema.Queries) > 0 {
		queryNode := &Node{
			Name:   "Query",
			Kind:   "OBJECT",
			Fields: []Edge{},
		}
		for _, q := range schema.Queries {
			edge := convertGitHubTopLevelField(q)
			queryNode.Fields = append(queryNode.Fields, edge)
		}
		graph.Nodes["Query"] = queryNode
		graph.Roots = append(graph.Roots, "Query")
	}

	// Build Mutation root from mutations array
	if includeMutations && len(schema.Mutations) > 0 {
		mutationNode := &Node{
			Name:   "Mutation",
			Kind:   "OBJECT",
			Fields: []Edge{},
		}
		for _, m := range schema.Mutations {
			// For mutations, determine return type from returnFields
			edge := convertGitHubMutation(m)
			mutationNode.Fields = append(mutationNode.Fields, edge)
		}
		graph.Nodes["Mutation"] = mutationNode
		graph.Roots = append(graph.Roots, "Mutation")
	}

	return graph
}

func convertGitHubObject(obj GitHubTypeDef) *Node {
	node := &Node{
		Name:       obj.Name,
		Kind:       "OBJECT",
		Fields:     []Edge{},
		Implements: []string{},
	}

	for _, f := range obj.Fields {
		edge := Edge{
			Name:   f.Name,
			Target: cleanTypeName(f.Type),
			Arguments: []Arg{},
		}
		for _, a := range f.Arguments {
			edge.Arguments = append(edge.Arguments, Arg{
				Name: a.Name,
				Type: cleanTypeName(a.Type),
			})
		}
		node.Fields = append(node.Fields, edge)
	}

	for _, i := range obj.Implements {
		node.Implements = append(node.Implements, i.Name)
	}

	return node
}

func convertGitHubInterface(iface GitHubTypeDef) *Node {
	node := &Node{
		Name:   iface.Name,
		Kind:   "INTERFACE",
		Fields: []Edge{},
	}

	for _, f := range iface.Fields {
		edge := Edge{
			Name:      f.Name,
			Target:    cleanTypeName(f.Type),
			Arguments: []Arg{},
		}
		for _, a := range f.Arguments {
			edge.Arguments = append(edge.Arguments, Arg{
				Name: a.Name,
				Type: cleanTypeName(a.Type),
			})
		}
		node.Fields = append(node.Fields, edge)
	}

	return node
}

func convertGitHubTopLevelField(f GitHubFieldDef) Edge {
	target := cleanTypeName(f.Type)
	edge := Edge{
		Name:      f.Name,
		Target:    target,
		Arguments: []Arg{},
	}
	for _, a := range f.Args {
		edge.Arguments = append(edge.Arguments, Arg{
			Name: a.Name,
			Type: cleanTypeName(a.Type),
		})
	}
	return edge
}

func convertGitHubMutation(m GitHubFieldDef) Edge {
	// Determine the primary return type from returnFields
	// Usually the first non-scalar return field is the main payload
	var targetType string
	for _, rf := range m.ReturnFields {
		if !isScalarType(rf.Kind) {
			targetType = rf.Type
			break
		}
	}
	if targetType == "" && len(m.ReturnFields) > 0 {
		targetType = m.ReturnFields[0].Type
	}
	targetType = cleanTypeName(targetType)
	if targetType == "" {
		targetType = cleanTypeName(m.Type)
	}

	edge := Edge{
		Name:      m.Name,
		Target:    targetType,
		Arguments: []Arg{},
	}

	// Use inputFields as arguments for mutations
	for _, inputField := range m.InputFields {
		// Skip the "input" wrapper argument if present
		if inputField.Name == "input" && inputField.Type != "" {
			// The actual arguments are fields of the input type
			// For simplicity, we just note this is an input object
			edge.Arguments = append(edge.Arguments, Arg{
				Name: "input",
				Type: cleanTypeName(inputField.Type),
			})
		} else {
			edge.Arguments = append(edge.Arguments, Arg{
				Name: inputField.Name,
				Type: cleanTypeName(inputField.Type),
			})
		}
	}

	return edge
}

func cleanTypeName(t string) string {
	if t == "" {
		return ""
	}
	// Remove GraphQL type wrappers: [Type!]! -> Type
	t = strings.ReplaceAll(t, "!", "")
	t = strings.ReplaceAll(t, "[", "")
	t = strings.ReplaceAll(t, "]", "")
	return t
}

func unwrapType(t TypeRef) string {
	if t.Name != "" {
		return t.Name
	}
	if t.OfType != nil {
		return unwrapType(*t.OfType)
	}
	return ""
}

func isScalarType(kind string) bool {
	return kind == "scalars" || kind == "SCALAR" || kind == "enums" || kind == "ENUM"
}

func findPaths(graph *Graph, target string, maxDepth int) [][]string {
	var results [][]string

	for _, root := range graph.Roots {
		visited := make(map[string]bool)
		var current []string
		dfs(graph, root, target, current, visited, &results, 0, maxDepth)
	}

	return results
}

func dfs(graph *Graph, current, target string, path []string, visited map[string]bool, results *[][]string, depth, maxDepth int) {
	if depth > maxDepth {
		return
	}

	if visited[current] {
		return
	}

	node, exists := graph.Nodes[current]
	if !exists {
		return
	}

	newPath := append(path, current)
	visited[current] = true

	// Found target (but path must be longer than just the root itself)
	if current == target && len(newPath) > 1 {
		pathCopy := make([]string, len(newPath))
		copy(pathCopy, newPath)
		*results = append(*results, pathCopy)
		visited[current] = false
		return
	}

	// Explore fields
	for _, field := range node.Fields {
		fieldType := field.Target

		// Skip scalar types unless they're the target
		if isScalar(fieldType) && fieldType != target {
			continue
		}

		fieldStep := field.Name
		if len(field.Arguments) > 0 {
			args := formatArguments(field.Arguments)
			fieldStep = fmt.Sprintf("%s(%s)", field.Name, args)
		}

		pathWithField := append(newPath, fieldStep)

		if fieldType == target {
			pathCopy := make([]string, len(pathWithField))
			copy(pathCopy, pathWithField)
			*results = append(*results, pathCopy)
		} else {
			dfs(graph, fieldType, target, pathWithField, visited, results, depth+1, maxDepth)
		}
	}

	// Explore possible types (interfaces/unions)
	for _, subType := range node.PossibleTypes {
		if subType == target {
			pathCopy := make([]string, len(newPath))
			copy(pathCopy, newPath)
			*results = append(*results, append(pathCopy, subType))
		} else {
			dfs(graph, subType, target, newPath, visited, results, depth+1, maxDepth)
		}
	}

	visited[current] = false
}

var scalarTypes = map[string]bool{
	"String": true, "Int": true, "Float": true, "Boolean": true, "ID": true,
	"DateTime": true, "Date": true, "URI": true, "HTML": true,
	"X509Certificate": true, "GitObjectID": true, "GitSSHRemote": true,
	"GitTimestamp": true, "PreciseDateTime": true, "Base64String": true,
}

func isScalar(name string) bool {
	return scalarTypes[name]
}

func formatArguments(args []Arg) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprintf("%s: %s", a.Name, a.Type)
	}
	return strings.Join(parts, ", ")
}

func formatPath(path []string) string {
	if len(path) == 0 {
		return ""
	}

	var formatted []string
	for i, step := range path {
		if i == 0 {
			formatted = append(formatted, color.CyanString(step))
		} else if i == len(path)-1 {
			formatted = append(formatted, color.GreenString(step))
		} else {
			formatted = append(formatted, step)
		}
	}

	return strings.Join(formatted, " → ")
}

