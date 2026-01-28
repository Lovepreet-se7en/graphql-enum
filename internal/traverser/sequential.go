package traverser

import (
	"github.com/lovepreet-se7en/graphql-enum/internal/schema"
)

type Sequential struct {
	schema   *schema.Schema
	maxDepth int
}

func NewSequential(s *schema.Schema, maxDepth int) *Sequential {
	return &Sequential{
		schema:   s,
		maxDepth: maxDepth,
	}
}

func (s *Sequential) FindPaths(entryPoints []schema.EntryPoint, targetType string) []schema.GraphQLPath {
	var paths []schema.GraphQLPath
	
	for _, ep := range entryPoints {
		visited := make(map[string]bool)
		visited[ep.Type] = true
		
		path := []schema.PathSegment{{Name: ep.Name, Type: ep.Type, Args: ep.Args}}
		
		if ep.Type == targetType {
			paths = append(paths, schema.GraphQLPath{
				Segments: path,
				Depth:    1,
			})
		}
		
		s.dfs(ep.Type, targetType, path, visited, 1, &paths)
	}
	
	return paths
}

func (s *Sequential) dfs(currentType, targetType string, currentPath []schema.PathSegment, 
	visited map[string]bool, depth int, paths *[]schema.GraphQLPath) {
	
	if depth >= s.maxDepth {
		return
	}
	
	typ := s.schema.GetType(currentType)
	if typ == nil {
		return
	}
	
	for _, field := range typ.Fields {
		if visited[field.Type] {
			continue
		}
		
		newSegment := schema.PathSegment{
			Name: field.Name,
			Type: field.Type,
			Args: field.Args,
		}
		
		newPath := append(currentPath, newSegment)
		
		if field.Type == targetType {
			*paths = append(*paths, schema.GraphQLPath{
				Segments: newPath,
				Depth:    depth + 1,
			})
			continue
		}
		
		newVisited := make(map[string]bool)
		for k, v := range visited {
			newVisited[k] = v
		}
		newVisited[field.Type] = true
		
		s.dfs(field.Type, targetType, newPath, newVisited, depth+1, paths)
	}
}
