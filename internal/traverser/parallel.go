package traverser

import (
	"sync"

	"github.com/lovepreet-se7en/graphql-enum/internal/schema"
)

type Parallel struct {
	schema     *schema.Schema
	maxDepth   int
	workers    int
}

type job struct {
	currentType string
	path        []schema.PathSegment
	visited     map[string]bool
	depth       int
}

func NewParallel(s *schema.Schema, maxDepth, workers int) *Parallel {
	if workers <= 0 {
		workers = 4
	}
	return &Parallel{
		schema:   s,
		maxDepth: maxDepth,
		workers:  workers,
	}
}

func (p *Parallel) FindPaths(entryPoints []schema.EntryPoint, targetType string) []schema.GraphQLPath {
	var (
		paths      []schema.GraphQLPath
		pathsMutex sync.Mutex
		wg         sync.WaitGroup
		jobs       = make(chan job, 1000)
	)

	// Start workers
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				localPaths := p.processJob(j, targetType)
				if len(localPaths) > 0 {
					pathsMutex.Lock()
					paths = append(paths, localPaths...)
					pathsMutex.Unlock()
				}
			}
		}()
	}

	// Seed initial jobs
	for _, ep := range entryPoints {
		visited := map[string]bool{ep.Type: true}
		initialPath := []schema.PathSegment{{Name: ep.Name, Type: ep.Type, Args: ep.Args}}
		
		if ep.Type == targetType {
			paths = append(paths, schema.GraphQLPath{
				Segments: initialPath,
				Depth:    1,
			})
		}
		
		jobs <- job{
			currentType: ep.Type,
			path:        initialPath,
			visited:     visited,
			depth:       1,
		}
	}

	close(jobs)
	wg.Wait()
	
	return paths
}

func (p *Parallel) processJob(j job, targetType string) []schema.GraphQLPath {
	if j.depth >= p.maxDepth {
		return nil
	}
	
	typ := p.schema.GetType(j.currentType)
	if typ == nil {
		return nil
	}
	
	var results []schema.GraphQLPath
	
	for _, field := range typ.Fields {
		if j.visited[field.Type] {
			continue
		}
		
		newSegment := schema.PathSegment{
			Name: field.Name,
			Type: field.Type,
			Args: field.Args,
		}
		
		newPath := make([]schema.PathSegment, len(j.path))
		copy(newPath, j.path)
		newPath = append(newPath, newSegment)
		
		if field.Type == targetType {
			results = append(results, schema.GraphQLPath{
				Segments: newPath,
				Depth:    j.depth + 1,
			})
			continue
		}
		
		// For parallel execution, only go deeper if depth is small to avoid explosion
		if j.depth < 5 {
			newVisited := make(map[string]bool)
			for k, v := range j.visited {
				newVisited[k] = v
			}
			newVisited[field.Type] = true
			
			subResults := p.processJob(job{
				currentType: field.Type,
				path:        newPath,
				visited:     newVisited,
				depth:       j.depth + 1,
			}, targetType)
			results = append(results, subResults...)
		}
	}
	
	return results
}
