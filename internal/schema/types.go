package schema

import "strings"

// Arg represents a GraphQL argument
type Arg struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Required     bool    `json:"required"`
	DefaultValue *string `json:"defaultValue"`
}

// Field represents a GraphQL field
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Args []Arg  `json:"args"`
}

// Type represents a GraphQL type
type Type struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Fields []Field `json:"fields"`
}

// EntryPoint represents a Query or Mutation entry point
type EntryPoint struct {
	Name string
	Type string
	Args []Arg
}

// PathSegment represents a segment in a path
type PathSegment struct {
	Name string
	Type string
	Args []Arg
}

// GraphQLPath represents a complete path from entry to target
type GraphQLPath struct {
	Segments []PathSegment
	Depth    int
}

// Schema holds the GraphQL schema
type Schema struct {
	Types       map[string]*Type
	QueryType   string
	MutationType string
}

// TypeExists checks if a type exists
func (s *Schema) TypeExists(name string) bool {
	_, exists := s.Types[name]
	return exists
}

// GetType retrieves a type by name
func (s *Schema) GetType(name string) *Type {
	return s.Types[name]
}

// FindSimilarTypes finds types with similar names
func (s *Schema) FindSimilarTypes(name string) []string {
	var similar []string
	nameLower := strings.ToLower(name)
	for typeName := range s.Types {
		if strings.Contains(strings.ToLower(typeName), nameLower) || 
		   strings.Contains(nameLower, strings.ToLower(typeName)) {
			similar = append(similar, typeName)
			if len(similar) >= 5 {
				break
			}
		}
	}
	return similar
}

// GetEntryPoints returns Query and optionally Mutation entry points
func (s *Schema) GetEntryPoints(includeMutations bool) []EntryPoint {
	var entries []EntryPoint
	
	if query := s.Types[s.QueryType]; query != nil {
		for _, f := range query.Fields {
			entries = append(entries, EntryPoint{
				Name: f.Name,
				Type: f.Type,
				Args: f.Args,
			})
		}
	}
	
	if includeMutations {
		if mutation := s.Types[s.MutationType]; mutation != nil {
			for _, f := range mutation.Fields {
				entries = append(entries, EntryPoint{
					Name: f.Name,
					Type: f.Type,
					Args: f.Args,
				})
			}
		}
	}
	
	return entries
}
