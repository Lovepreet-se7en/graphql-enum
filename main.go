package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// Introspection Schema Types
type IntrospectionFormat struct {
	Data struct {
		Schema struct {
			QueryType        *NamedType `json:"queryType"`
			MutationType     *NamedType `json:"mutationType"`
			SubscriptionType *NamedType `json:"subscriptionType"`
			Types            []FullType `json:"types"`
		} `json:"__schema"`
	} `json:"data"`
}

type NamedType struct {
	Name string `json:"name"`
}

type FullType struct {
	Kind          string             `json:"kind"`
	Name          string             `json:"name"`
	Fields        []IntrospectionField `json:"fields"`
	Interfaces    []NamedType        `json:"interfaces"`
	PossibleTypes []NamedType        `json:"possibleTypes"`
}

type IntrospectionField struct {
	Name string             `json:"name"`
	Type TypeRef            `json:"type"`
	Args []IntrospectionArg `json:"args"`
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

// GitHub documentation format
type GitHubType string

func (gt *GitHubType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*gt = GitHubType(s)
		return nil
	}

	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		*gt = GitHubType(obj.Name)
		return nil
	}
	return nil
}

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
	Type         GitHubType        `json:"type"`
	Kind         string            `json:"kind"`
	ID           string            `json:"id"`
	Href         string            `json:"href"`
	Description  string            `json:"description"`
	Args         []GitHubArg       `json:"args"`         // Top-level queries/mutations use "args"
	InputFields  []GitHubFieldDef  `json:"inputFields"`  // Mutations use inputFields
	ReturnFields []GitHubReturnField `json:"returnFields"` // Mutations have returnFields
}

type GitHubArg struct {
	Name         string      `json:"name"`
	Type         GitHubType  `json:"type"`
	ID           string      `json:"id"`
	Kind         string      `json:"kind"`
	Href         string      `json:"href"`
	Description  string      `json:"description"`
	DefaultValue interface{} `json:"defaultValue,omitempty"`
}

type GitHubReturnField struct {
	Name        string     `json:"name"`
	Type        GitHubType `json:"type"`
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Description string     `json:"description"`
	IsDeprecated bool      `json:"isDeprecated,omitempty"`
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

type GitHubField struct {
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	Type              GitHubType  `json:"type"`
	ID                string      `json:"id"`
	Kind              string      `json:"kind"`
	Href              string      `json:"href"`
	Arguments         []GitHubArg `json:"arguments"`
	IsDeprecated      bool        `json:"isDeprecated,omitempty"`
	DeprecationReason string      `json:"deprecationReason,omitempty"`
}

type GitHubImplement struct {
	Name GitHubType `json:"name"`
	ID   string     `json:"id"`
	Href string     `json:"href"`
}

func (gi *GitHubImplement) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		gi.Name = GitHubType(name)
		return nil
	}
	type Alias GitHubImplement
	var aux struct {
		Name GitHubType `json:"name"`
		*Alias
	}
	aux.Alias = (*Alias)(gi)
	if err := json.Unmarshal(data, &aux); err == nil {
		gi.Name = aux.Name
		return nil
	}
	return nil
}

type GitHubUnion struct {
	Name          string   `json:"name"`
	Kind          string   `json:"kind"`
	ID            string   `json:"id"`
	Href          string   `json:"href"`
	Description   string   `json:"description"`
	PossibleTypes []string `json:"-"`
}

func (u *GitHubUnion) UnmarshalJSON(data []byte) error {
	type Alias GitHubUnion
	aux := struct {
		PossibleTypes []interface{} `json:"possibleTypes"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	for _, pt := range aux.PossibleTypes {
		switch v := pt.(type) {
		case string:
			u.PossibleTypes = append(u.PossibleTypes, v)
		case map[string]interface{}:
			if name, ok := v["name"].(string); ok {
				u.PossibleTypes = append(u.PossibleTypes, name)
			}
		}
	}
	return nil
}

type GitHubEnum struct {
	Name string `json:"name"`
}

type GitHubScalar struct {
	Name string `json:"name"`
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
		schemaFile       = flag.String("schema", "", "Path to schema JSON file")
		targetType       = flag.String("type", "", "Target type name to find paths to")
		maxDepth         = flag.Int("max-depth", 15, "Maximum traversal depth")
		limit            = flag.Int("limit", 0, "Stop after finding N paths (0 for no limit)")
		includeMutations = flag.Bool("mutations", false, "Include Mutation fields as entry points")
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

	data, err := os.ReadFile(*schemaFile)
	if err != nil {
		fmt.Printf("Error reading schema file: %v\n", err)
		os.Exit(1)
	}

	graph, format, err := parseSchema(data, *includeMutations)
	if err != nil {
		fmt.Printf("Parse Error: %v\n", err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Loaded schema using %s format\n", color.CyanString(format))
	}

	targetNode, exists := graph.Nodes[*targetType]
	if !exists {
		fmt.Printf("Error: Target type '%s' not found in schema.\n", color.RedString(*targetType))
		suggestions := findSimilarTypes(graph, *targetType)
		if len(suggestions) > 0 {
			fmt.Printf("Did you mean: %s?\n", strings.Join(suggestions, ", "))
		}
		os.Exit(1)
	}

	fmt.Printf("Target: %s (%s)\n", color.GreenString(*targetType), targetNode.Kind)
	fmt.Printf("Entry points: %s\n", strings.Join(graph.Roots, ", "))
	fmt.Printf("Max depth: %d\n", *maxDepth)
	if *limit > 0 {
		fmt.Printf("Limit: %d paths\n", *limit)
	}
	fmt.Println()

	paths := findPaths(graph, *targetType, *maxDepth, *limit)

	if len(paths) == 0 {
		fmt.Printf("No paths found to type '%s' with max depth %d.\n", *targetType, *maxDepth)
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
	fmt.Println("Usage: graphql-enum -schema <file.json> -type <TypeName> [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
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
	var intro IntrospectionFormat
	if err := json.Unmarshal(data, &intro); err == nil && intro.Data.Schema.Types != nil {
		graph := buildFromIntrospection(&intro, includeMutations)
		return graph, "GraphQL Introspection", nil
	}

	var gh GitHubFormat
	if err := json.Unmarshal(data, &gh); err == nil {
		if len(gh.Queries) > 0 || len(gh.Objects) > 0 {
			graph := buildFromGitHubFormat(&gh, includeMutations)
			return graph, "GitHub Schema Format", nil
		}
	} else {
		// fmt.Printf("DEBUG: GitHub Unmarshal Error: %v\n", err)
	}

	return nil, "", fmt.Errorf("unknown schema format")
}

func buildFromIntrospection(schema *IntrospectionFormat, includeMutations bool) *Graph {
	graph := &Graph{Nodes: make(map[string]*Node), Roots: []string{}}
	for _, t := range schema.Data.Schema.Types {
		if strings.HasPrefix(t.Name, "__") {
			continue
		}
		node := &Node{Name: t.Name, Kind: t.Kind, Fields: []Edge{}}
		for _, f := range t.Fields {
			targetType := unwrapType(f.Type)
			if targetType == "" {
				continue
			}
			edge := Edge{Name: f.Name, Target: targetType}
			for _, a := range f.Args {
				edge.Arguments = append(edge.Arguments, Arg{Name: a.Name, Type: unwrapType(a.Type)})
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
	return graph
}

func buildFromGitHubFormat(schema *GitHubFormat, includeMutations bool) *Graph {
	graph := &Graph{Nodes: make(map[string]*Node), Roots: []string{}}
	for _, obj := range schema.Objects {
		graph.Nodes[obj.Name] = convertGitHubTypeDef(obj, "OBJECT")
	}
	for _, iface := range schema.Interfaces {
		graph.Nodes[iface.Name] = convertGitHubTypeDef(iface, "INTERFACE")
	}
	for _, union := range schema.Unions {
		graph.Nodes[union.Name] = &Node{Name: union.Name, Kind: "UNION", PossibleTypes: union.PossibleTypes}
	}
	for _, enum := range schema.Enums {
		graph.Nodes[enum.Name] = &Node{Name: enum.Name, Kind: "ENUM"}
	}
	for _, scalar := range schema.Scalars {
		graph.Nodes[scalar.Name] = &Node{Name: scalar.Name, Kind: "SCALAR"}
	}
	for _, node := range graph.Nodes {
		for _, ifaceName := range node.Implements {
			if ifaceNode, exists := graph.Nodes[ifaceName]; exists {
				ifaceNode.PossibleTypes = append(ifaceNode.PossibleTypes, node.Name)
			}
		}
	}
	queryNode := &Node{Name: "Query", Kind: "OBJECT", Fields: []Edge{}}
	for _, q := range schema.Queries {
		queryNode.Fields = append(queryNode.Fields, convertGitHubFieldDef(q))
	}
	graph.Nodes["Query"] = queryNode
	graph.Roots = append(graph.Roots, "Query")
	if includeMutations && len(schema.Mutations) > 0 {
		mutNode := &Node{Name: "Mutation", Kind: "OBJECT", Fields: []Edge{}}
		for _, m := range schema.Mutations {
			mutNode.Fields = append(mutNode.Fields, convertGitHubMutation(m))
		}
		graph.Nodes["Mutation"] = mutNode
		graph.Roots = append(graph.Roots, "Mutation")
	}
	return graph
}

func convertGitHubTypeDef(obj GitHubTypeDef, kind string) *Node {
	node := &Node{Name: obj.Name, Kind: kind, Fields: []Edge{}, Implements: []string{}}
	for _, impl := range obj.Implements {
		node.Implements = append(node.Implements, string(impl.Name))
	}
	for _, f := range obj.Fields {
		edge := Edge{Name: f.Name, Target: cleanTypeName(string(f.Type))}
		for _, arg := range f.Arguments {
			edge.Arguments = append(edge.Arguments, Arg{Name: arg.Name, Type: cleanTypeName(string(arg.Type))})
		}
		node.Fields = append(node.Fields, edge)
	}
	return node
}

func convertGitHubFieldDef(f GitHubFieldDef) Edge {
	edge := Edge{Name: f.Name, Target: cleanTypeName(string(f.Type))}
	for _, arg := range f.Args {
		edge.Arguments = append(edge.Arguments, Arg{Name: arg.Name, Type: cleanTypeName(string(arg.Type))})
	}
	return edge
}

func convertGitHubMutation(m GitHubFieldDef) Edge {
	targetType := string(m.Type)
	if len(m.ReturnFields) > 0 {
		for _, rf := range m.ReturnFields {
			if strings.Contains(strings.ToLower(rf.Name), strings.ToLower(m.Name)) ||
					(rf.Kind == "objects" && !strings.HasSuffix(rf.Name, "Edge") && !strings.Contains(rf.Name, "Connection")) {
				targetType = string(rf.Type)
				break
			}
		}
	}
	edge := Edge{Name: m.Name, Target: cleanTypeName(targetType)}
	for _, inputField := range m.InputFields {
		edge.Arguments = append(edge.Arguments, Arg{Name: inputField.Name, Type: cleanTypeName(string(inputField.Type))})
	}
	return edge
}

func cleanTypeName(t string) string {
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

func isScalar(graph *Graph, name string) bool {
	if s := map[string]bool{"String": true, "Int": true, "Float": true, "Boolean": true, "ID": true}[name]; s {
		return true
	}
	if node, exists := graph.Nodes[name]; exists {
		return node.Kind == "SCALAR" || node.Kind == "ENUM"
	}
	return false
}

func findPaths(graph *Graph, target string, maxDepth, limit int) [][]string {
	var results [][]string
	for _, root := range graph.Roots {
		visited := make(map[string]bool)
		dfs(graph, root, target, []string{root}, visited, &results, 0, maxDepth, limit)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results
}

func dfs(graph *Graph, current, target string, path []string, visited map[string]bool, results *[][]string, depth, maxDepth, limit int) {
	if depth > maxDepth || (limit > 0 && len(*results) >= limit) || visited[current] {
		return
	}
	node, exists := graph.Nodes[current]
	if !exists {
		return
	}
	visited[current] = true
	defer func() { visited[current] = false }()

	for _, field := range node.Fields {
		if isScalar(graph, field.Target) && field.Target != target {
			continue
		}
		fieldStep := field.Name
		if len(field.Arguments) > 0 {
			var args []string
			for _, a := range field.Arguments {
				args = append(args, fmt.Sprintf("%s: %s", a.Name, a.Type))
			}
			fieldStep = fmt.Sprintf("%s(%s)", field.Name, strings.Join(args, ", "))
		}
		newPath := append([]string{}, path...)
		newPath = append(newPath, fieldStep)
		if field.Target == target {
			if limit > 0 && len(*results) >= limit {
				return
			}
			*results = append(*results, append(newPath, target))
			if limit > 0 && len(*results) >= limit {
				return
			}
		} else {
			dfs(graph, field.Target, target, newPath, visited, results, depth+1, maxDepth, limit)
		}
	}
	for _, subType := range node.PossibleTypes {
		newPath := append([]string{}, path...)
		newPath = append(newPath, subType)
		if subType == target {
			if limit > 0 && len(*results) >= limit {
				return
			}
			*results = append(*results, newPath)
			if limit > 0 && len(*results) >= limit {
				return
			}
		} else {
			dfs(graph, subType, target, newPath, visited, results, depth, maxDepth, limit)
		}
	}
}

func formatPath(path []string) string {
	var f []string
	for i, s := range path {
		if i == 0 {
			f = append(f, color.CyanString(s))
		} else if i == len(path)-1 {
			f = append(f, color.GreenString(s))
		} else {
			f = append(f, s)
		}
	}
	return strings.Join(f, " → ")
}