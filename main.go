package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/lovepreet-se7en/graphql-enum/internal/generator"
	"github.com/lovepreet-se7en/graphql-enum/internal/schema"
	"github.com/lovepreet-se7en/graphql-enum/internal/traverser"
	"github.com/lovepreet-se7en/graphql-enum/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
)

var (
	schemaFile  = flag.String("schema", "", "Path to schema JSON file (required)")
	targetType  = flag.String("type", "", "Target type name to find paths to (required)")
	maxDepth    = flag.Int("max-depth", 15, "Maximum traversal depth")
	mutations   = flag.Bool("mutations", false, "Include Mutation fields as entry points")
	verbose     = flag.Bool("v", false, "Verbose output")
	noColor     = flag.Bool("no-color", false, "Disable colored output")
	interactive = flag.Bool("i", false, "Interactive TUI mode")
	generate    = flag.Bool("generate", false, "Generate executable queries")
	outputDir   = flag.String("output", "./queries", "Output directory for generated queries")
	endpoint    = flag.String("endpoint", "", "GraphQL endpoint for curl generation")
	parallel    = flag.Int("parallel", 0, "Number of parallel workers (0=sequential)")
	jsonExport  = flag.String("json", "", "Export results to JSON file")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: graphql-enum -schema <file> -type <name> [options]\n\n")
		fmt.Fprintf(os.Stderr, "GraphQL Path Enumeration Tool - Lists all possible paths from root queries/mutations to a target type\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  graphql-enum -schema schema.json -type User\n")
		fmt.Fprintf(os.Stderr, "  graphql-enum -schema schema.json -type User -i -parallel 8\n")
		fmt.Fprintf(os.Stderr, "  graphql-enum -schema schema.json -type Repository -generate -endpoint https://api.example.com/graphql\n")
	}
	flag.Parse()

	if *noColor {
		color.NoColor = true
	}

	// Validation
	if *schemaFile == "" || *targetType == "" {
		flag.Usage()
		os.Exit(1)
	}

	if _, err := os.Stat(*schemaFile); os.IsNotExist(err) {
		log.Fatalf("Schema file not found: %s", *schemaFile)
	}

	// Load schema
	if *verbose {
		color.Cyan("Loading schema from %s...", *schemaFile)
	}

	scm, err := schema.Load(*schemaFile)
	if err != nil {
		log.Fatalf("Failed to load schema: %v", err)
	}

	if *verbose {
		color.Green("Schema loaded: %d types found", len(scm.Types))
	}

	// Validate target type exists
	if !scm.TypeExists(*targetType) {
		suggestions := scm.FindSimilarTypes(*targetType)
		fmt.Fprintf(os.Stderr, color.RedString("Error: Type '%s' not found in schema\n", *targetType))
		if len(suggestions) > 0 {
			fmt.Fprintf(os.Stderr, color.YellowString("\nDid you mean:\n"))
			for _, s := range suggestions {
				fmt.Fprintf(os.Stderr, "  - %s\n", s)
			}
		}
		os.Exit(1)
	}

	// Get entry points
	entryPoints := scm.GetEntryPoints(*mutations)
	if len(entryPoints) == 0 {
		log.Fatal("No entry points found in schema")
	}

	if *verbose {
		color.Cyan("Searching %d entry points...", len(entryPoints))
	}

	// Choose traversal strategy
	var paths []schema.GraphQLPath
	
	if *parallel > 0 {
		if *verbose {
			color.Cyan("Using parallel traversal with %d workers...", *parallel)
		}
		pt := traverser.NewParallel(scm, *maxDepth, *parallel)
		paths = pt.FindPaths(entryPoints, *targetType)
	} else {
		if *verbose {
			color.Cyan("Using sequential traversal...")
		}
		st := traverser.NewSequential(scm, *maxDepth)
		paths = st.FindPaths(entryPoints, *targetType)
	}

	if len(paths) == 0 {
		color.Yellow("No paths found to %s (try increasing -max-depth)", *targetType)
		os.Exit(0)
	}

	// Handle output modes
	switch {
	case *interactive:
		runInteractive(paths, scm, *targetType)
	case *generate:
		generateQueries(paths, scm, *targetType)
	case *jsonExport != "":
		exportJSON(paths, *jsonExport)
	default:
		printPaths(paths, *targetType)
	}
}

func runInteractive(paths []schema.GraphQLPath, scm *schema.Schema, target string) {
	m := tui.NewModel(paths, scm, target)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

func generateQueries(paths []schema.GraphQLPath, scm *schema.Schema, target string) {
	gen := generator.New(scm, *outputDir)
	queries, err := gen.GenerateAll(paths)
	if err != nil {
		log.Fatalf("Failed to generate queries: %v", err)
	}

	if err := gen.SaveToFiles(queries); err != nil {
		log.Fatalf("Failed to save queries: %v", err)
	}

	color.Green("✓ Generated %d queries in %s/", len(queries), *outputDir)

	if *endpoint != "" {
		cmds := gen.GenerateCurlCommands(*endpoint, queries)
		curlFile := filepath.Join(*outputDir, "test_commands.sh")
		content := "#!/bin/bash\n\n# Auto-generated GraphQL test commands\n# Endpoint: " + *endpoint + "\n\n"
		content += strings.Join(cmds, "\n\n")
		content += "\n"
		
		if err := os.WriteFile(curlFile, []byte(content), 0755); err != nil {
			log.Printf("Warning: Failed to save curl commands: %v", err)
		} else {
			color.Green("✓ Curl commands saved to %s", curlFile)
		}
	}
}

func exportJSON(paths []schema.GraphQLPath, filename string) {
	type pathExport struct {
		Path      string   `json:"path"`
		Depth     int      `json:"depth"`
		Segments  []string `json:"segments"`
		Query     string   `json:"query,omitempty"`
	}

	exports := make([]pathExport, len(paths))
	for i, p := range paths {
		segments := make([]string, len(p.Segments))
		for j, s := range p.Segments {
			segments[j] = s.Name
		}
		exports[i] = pathExport{
			Path:     formatPath(p),
			Depth:    p.Depth,
			Segments: segments,
		}
	}

	data := struct {
		Count   int           `json:"count"`
		Target  string        `json:"target"`
		Paths   []pathExport  `json:"paths"`
	}{
		Count:  len(paths),
		Target: *targetType,
		Paths:  exports,
	}

	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(filename, file, 0644); err != nil {
		log.Fatalf("Failed to write JSON: %v", err)
	}

	color.Green("✓ Exported %d paths to %s", len(paths), filename)
}

func printPaths(paths []schema.GraphQLPath, target string) {
	cyan := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen)
	white := color.New(color.FgWhite)

	cyan.Printf("\nTarget: %s\n", target)
	green.Printf("Found %d paths:\n\n", len(paths))

	for i, path := range paths {
		white.Printf("  %d. ", i+1)
		fmt.Println(formatPath(path))
	}
	fmt.Println()
}

func formatPath(path schema.GraphQLPath) string {
	var parts []string
	for _, seg := range path.Segments {
		if len(seg.Args) > 0 {
			args := make([]string, len(seg.Args))
			for i, a := range seg.Args {
				args[i] = fmt.Sprintf("%s: %s", a.Name, a.Type)
			}
			parts = append(parts, fmt.Sprintf("%s(%s)", seg.Name, strings.Join(args, ", ")))
		} else {
			parts = append(parts, seg.Name)
		}
	}
	return strings.Join(parts, " → ")
}
