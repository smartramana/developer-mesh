package parsers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/chunking"
)

// GoParser is a parser for Go code
type GoParser struct{}

// NewGoParser creates a new GoParser
func NewGoParser() *GoParser {
	return &GoParser{}
}

// GetLanguage returns the language this parser handles
func (p *GoParser) GetLanguage() chunking.Language {
	return chunking.LanguageGo
}

// Parse parses Go code and returns chunks
func (p *GoParser) Parse(ctx context.Context, code string, filename string) ([]*chunking.CodeChunk, error) {
	// Parse the Go code into an AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, code, parser.ParseComments)
	if err != nil {
		return p.fallbackParse(code, filename), nil
	}

	chunks := []*chunking.CodeChunk{}

	// Extract package information
	packageName := file.Name.Name
	packageChunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageGo,
		StartLine: 1,
		EndLine:   countLines(code),
		Metadata: map[string]interface{}{
			"package": packageName,
		},
	}

	// Generate ID for package chunk
	packageChunk.ID = generateChunkID(packageChunk)
	chunks = append(chunks, packageChunk)

	// Track imports
	importMap := make(map[string]string)
	importChunks := []*chunking.CodeChunk{}

	// Extract imports
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		var importName string

		if imp.Name != nil {
			importName = imp.Name.Name
		} else {
			parts := strings.Split(importPath, "/")
			importName = parts[len(parts)-1]
		}

		importMap[importName] = importPath

		// Create import chunk
		importChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeImport,
			Name:      importName,
			Path:      fmt.Sprintf("%s:import:%s", packageName, importName),
			Content:   importPath,
			Language:  chunking.LanguageGo,
			StartLine: fset.Position(imp.Pos()).Line,
			EndLine:   fset.Position(imp.End()).Line,
			ParentID:  packageChunk.ID,
			Metadata: map[string]interface{}{
				"import_path": importPath,
				"import_name": importName,
			},
		}

		// Generate ID
		importChunk.ID = generateChunkID(importChunk)
		importChunks = append(importChunks, importChunk)
	}

	// Add import chunks to the list
	chunks = append(chunks, importChunks...)

	// Extract comments
	for _, cg := range file.Comments {
		commentChunk := &chunking.CodeChunk{
			Type:      chunking.ChunkTypeComment,
			Name:      "Comment",
			Path:      fmt.Sprintf("%s:comment:%d", packageName, fset.Position(cg.Pos()).Line),
			Content:   cg.Text(),
			Language:  chunking.LanguageGo,
			StartLine: fset.Position(cg.Pos()).Line,
			EndLine:   fset.Position(cg.End()).Line,
			ParentID:  packageChunk.ID,
		}

		// Generate ID
		commentChunk.ID = generateChunkID(commentChunk)
		chunks = append(chunks, commentChunk)
	}

	// Process all declarations
	funcChunks := []*chunking.CodeChunk{}
	typeChunks := []*chunking.CodeChunk{}

	// Extract type declarations
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.TypeSpec:
			chunk := p.processTypeSpec(decl, fset, packageName, packageChunk.ID, code)
			if chunk != nil {
				typeChunks = append(typeChunks, chunk)
			}

		case *ast.FuncDecl:
			chunk := p.processFuncDecl(decl, fset, packageName, packageChunk.ID, typeChunks, code)
			if chunk != nil {
				funcChunks = append(funcChunks, chunk)
			}
		}
		return true
	})

	// Add type and function chunks to the list
	chunks = append(chunks, typeChunks...)
	chunks = append(chunks, funcChunks...)

	// Process dependencies between chunks
	p.processDependencies(chunks, importMap)

	return chunks, nil
}

// processTypeSpec processes a type declaration and returns a chunk
func (p *GoParser) processTypeSpec(typeSpec *ast.TypeSpec, fset *token.FileSet, packageName string, parentID string, code string) *chunking.CodeChunk {
	var chunkType chunking.ChunkType
	var typeDetails map[string]interface{}

	switch typeSpec.Type.(type) {
	case *ast.StructType:
		chunkType = chunking.ChunkTypeStruct
		typeDetails = p.extractStructDetails(typeSpec.Type.(*ast.StructType))
	case *ast.InterfaceType:
		chunkType = chunking.ChunkTypeInterface
		typeDetails = p.extractInterfaceDetails(typeSpec.Type.(*ast.InterfaceType))
	default:
		// Skip other type declarations
		return nil
	}

	startPos := fset.Position(typeSpec.Pos())
	endPos := fset.Position(typeSpec.End())

	// Extract the content of the type declaration
	lines := strings.Split(code, "\n")
	contentLines := lines[startPos.Line-1 : endPos.Line]
	content := strings.Join(contentLines, "\n")

	chunk := &chunking.CodeChunk{
		Type:      chunkType,
		Name:      typeSpec.Name.Name,
		Path:      fmt.Sprintf("%s.%s", packageName, typeSpec.Name.Name),
		Content:   content,
		Language:  chunking.LanguageGo,
		StartLine: startPos.Line,
		EndLine:   endPos.Line,
		ParentID:  parentID,
		Metadata:  typeDetails,
	}

	// Generate ID
	chunk.ID = generateChunkID(chunk)

	return chunk
}

// processFuncDecl processes a function declaration and returns a chunk
func (p *GoParser) processFuncDecl(funcDecl *ast.FuncDecl, fset *token.FileSet, packageName string, parentID string, typeChunks []*chunking.CodeChunk, code string) *chunking.CodeChunk {
	startPos := fset.Position(funcDecl.Pos())
	endPos := fset.Position(funcDecl.End())

	// Extract the content of the function declaration
	lines := strings.Split(code, "\n")
	contentLines := lines[startPos.Line-1 : endPos.Line]
	content := strings.Join(contentLines, "\n")

	// Determine if this is a method or a standalone function
	chunkType := chunking.ChunkTypeFunction
	var path string
	methodParentID := parentID

	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		chunkType = chunking.ChunkTypeMethod

		// Try to extract the receiver type
		var receiverType string
		switch t := funcDecl.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				receiverType = ident.Name
			}
		case *ast.Ident:
			receiverType = t.Name
		}

		if receiverType != "" {
			path = fmt.Sprintf("%s.%s.%s", packageName, receiverType, funcDecl.Name.Name)

			// Find the parent type chunk
			for _, typeChunk := range typeChunks {
				if typeChunk.Name == receiverType {
					methodParentID = typeChunk.ID
					break
				}
			}
		} else {
			path = fmt.Sprintf("%s.%s", packageName, funcDecl.Name.Name)
		}
	} else {
		path = fmt.Sprintf("%s.%s", packageName, funcDecl.Name.Name)
	}

	// Extract function details
	funcDetails := p.extractFunctionDetails(funcDecl)

	chunk := &chunking.CodeChunk{
		Type:      chunkType,
		Name:      funcDecl.Name.Name,
		Path:      path,
		Content:   content,
		Language:  chunking.LanguageGo,
		StartLine: startPos.Line,
		EndLine:   endPos.Line,
		ParentID:  methodParentID,
		Metadata:  funcDetails,
	}

	// Generate ID
	chunk.ID = generateChunkID(chunk)

	return chunk
}

// extractStructDetails extracts fields and other information from a struct
func (p *GoParser) extractStructDetails(structType *ast.StructType) map[string]interface{} {
	details := map[string]interface{}{
		"type":   "struct",
		"fields": []map[string]string{},
	}

	if structType.Fields != nil {
		for _, field := range structType.Fields.List {
			fieldType := ""

			// Extract the field type as a string
			switch t := field.Type.(type) {
			case *ast.Ident:
				fieldType = t.Name
			case *ast.SelectorExpr:
				if x, ok := t.X.(*ast.Ident); ok {
					fieldType = x.Name + "." + t.Sel.Name
				}
			case *ast.StarExpr:
				// Handle pointer types
				if ident, ok := t.X.(*ast.Ident); ok {
					fieldType = "*" + ident.Name
				} else if sel, ok := t.X.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok {
						fieldType = "*" + x.Name + "." + sel.Sel.Name
					}
				}
			case *ast.ArrayType:
				fieldType = "[]"
				if ident, ok := t.Elt.(*ast.Ident); ok {
					fieldType += ident.Name
				}
			case *ast.MapType:
				fieldType = "map"
			case *ast.InterfaceType:
				fieldType = "interface{}"
			}

			// Extract field names
			for _, name := range field.Names {
				fieldInfo := map[string]string{
					"name": name.Name,
					"type": fieldType,
				}

				// Extract tags if present
				if field.Tag != nil {
					fieldInfo["tag"] = field.Tag.Value
				}

				fields := details["fields"].([]map[string]string)
				fields = append(fields, fieldInfo)
				details["fields"] = fields
			}
		}
	}

	return details
}

// extractInterfaceDetails extracts methods and other information from an interface
func (p *GoParser) extractInterfaceDetails(interfaceType *ast.InterfaceType) map[string]interface{} {
	details := map[string]interface{}{
		"type":    "interface",
		"methods": []map[string]string{},
	}

	if interfaceType.Methods != nil {
		for _, method := range interfaceType.Methods.List {
			// Check if it's a method
			if len(method.Names) > 0 {
				methodType := ""

				// Try to extract method signature
				if funcType, ok := method.Type.(*ast.FuncType); ok {
					params := []string{}
					returns := []string{}

					// Extract parameters
					if funcType.Params != nil {
						for _, param := range funcType.Params.List {
							typeStr := ""
							switch t := param.Type.(type) {
							case *ast.Ident:
								typeStr = t.Name
							case *ast.SelectorExpr:
								if x, ok := t.X.(*ast.Ident); ok {
									typeStr = x.Name + "." + t.Sel.Name
								}
							}

							if len(param.Names) > 0 {
								for _, name := range param.Names {
									params = append(params, name.Name+" "+typeStr)
								}
							} else {
								params = append(params, typeStr)
							}
						}
					}

					// Extract return values
					if funcType.Results != nil {
						for _, result := range funcType.Results.List {
							typeStr := ""
							switch t := result.Type.(type) {
							case *ast.Ident:
								typeStr = t.Name
							case *ast.SelectorExpr:
								if x, ok := t.X.(*ast.Ident); ok {
									typeStr = x.Name + "." + t.Sel.Name
								}
							}
							returns = append(returns, typeStr)
						}
					}

					methodType = "func(" + strings.Join(params, ", ") + ")"
					if len(returns) > 0 {
						if len(returns) == 1 {
							methodType += " " + returns[0]
						} else {
							methodType += " (" + strings.Join(returns, ", ") + ")"
						}
					}
				}

				for _, name := range method.Names {
					methodInfo := map[string]string{
						"name":      name.Name,
						"signature": methodType,
					}

					methods := details["methods"].([]map[string]string)
					methods = append(methods, methodInfo)
					details["methods"] = methods
				}
			}
		}
	}

	return details
}

// extractFunctionDetails extracts parameters, return types, and other information from a function
func (p *GoParser) extractFunctionDetails(funcDecl *ast.FuncDecl) map[string]interface{} {
	details := map[string]interface{}{
		"params":  []map[string]string{},
		"returns": []map[string]string{},
	}

	// Extract parameters
	if funcDecl.Type.Params != nil {
		for _, param := range funcDecl.Type.Params.List {
			typeStr := ""
			switch t := param.Type.(type) {
			case *ast.Ident:
				typeStr = t.Name
			case *ast.SelectorExpr:
				if x, ok := t.X.(*ast.Ident); ok {
					typeStr = x.Name + "." + t.Sel.Name
				}
			case *ast.StarExpr:
				// Handle pointer types
				if ident, ok := t.X.(*ast.Ident); ok {
					typeStr = "*" + ident.Name
				} else if sel, ok := t.X.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok {
						typeStr = "*" + x.Name + "." + sel.Sel.Name
					}
				}
			}

			for _, name := range param.Names {
				paramInfo := map[string]string{
					"name": name.Name,
					"type": typeStr,
				}

				params := details["params"].([]map[string]string)
				params = append(params, paramInfo)
				details["params"] = params
			}
		}
	}

	// Extract return types
	if funcDecl.Type.Results != nil {
		for _, result := range funcDecl.Type.Results.List {
			typeStr := ""
			switch t := result.Type.(type) {
			case *ast.Ident:
				typeStr = t.Name
			case *ast.SelectorExpr:
				if x, ok := t.X.(*ast.Ident); ok {
					typeStr = x.Name + "." + t.Sel.Name
				}
			case *ast.StarExpr:
				// Handle pointer types
				if ident, ok := t.X.(*ast.Ident); ok {
					typeStr = "*" + ident.Name
				} else if sel, ok := t.X.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok {
						typeStr = "*" + x.Name + "." + sel.Sel.Name
					}
				}
			}

			name := ""
			if len(result.Names) > 0 {
				name = result.Names[0].Name
			}

			returnInfo := map[string]string{
				"name": name,
				"type": typeStr,
			}

			returns := details["returns"].([]map[string]string)
			returns = append(returns, returnInfo)
			details["returns"] = returns
		}
	}

	// Add receiver info if this is a method
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		recv := funcDecl.Recv.List[0]
		recvType := ""

		switch t := recv.Type.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				recvType = "*" + ident.Name
			}
		case *ast.Ident:
			recvType = t.Name
		}

		recvName := ""
		if len(recv.Names) > 0 {
			recvName = recv.Names[0].Name
		}

		details["receiver"] = map[string]string{
			"name": recvName,
			"type": recvType,
		}
	}

	return details
}

// processDependencies analyzes chunks to identify dependencies between them
func (p *GoParser) processDependencies(chunks []*chunking.CodeChunk, importMap map[string]string) {
	// Map chunks by ID for quick lookup
	chunkMap := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}

	// Map chunks by name for dependency tracking
	chunksByName := make(map[string]*chunking.CodeChunk)
	for _, chunk := range chunks {
		if chunk.Type == chunking.ChunkTypeFunction ||
			chunk.Type == chunking.ChunkTypeMethod ||
			chunk.Type == chunking.ChunkTypeStruct ||
			chunk.Type == chunking.ChunkTypeInterface {
			chunksByName[chunk.Name] = chunk
		}
	}

	// Analyze dependencies
	for _, chunk := range chunks {
		// Skip non-code chunks
		if chunk.Type != chunking.ChunkTypeFunction &&
			chunk.Type != chunking.ChunkTypeMethod &&
			chunk.Type != chunking.ChunkTypeStruct &&
			chunk.Type != chunking.ChunkTypeInterface {
			continue
		}

		// Analyze content for references to other chunks
		for name, dependentChunk := range chunksByName {
			// Skip self-reference
			if name == chunk.Name {
				continue
			}

			// Check if the chunk content contains references to other chunks
			if strings.Contains(chunk.Content, name) {
				// Add the dependency
				if chunk.Dependencies == nil {
					chunk.Dependencies = []string{}
				}
				chunk.Dependencies = append(chunk.Dependencies, dependentChunk.ID)
			}
		}

		// Add parent dependency if it exists
		if chunk.ParentID != "" && chunk.ParentID != chunk.ID {
			if chunk.Dependencies == nil {
				chunk.Dependencies = []string{}
			}
			chunk.Dependencies = append(chunk.Dependencies, chunk.ParentID)
		}
	}
}

// fallbackParse provides a simple line-based chunking for Go code when AST parsing fails
func (p *GoParser) fallbackParse(code string, filename string) []*chunking.CodeChunk {
	lines := strings.Split(code, "\n")

	// Create a single chunk for the entire file
	chunk := &chunking.CodeChunk{
		Type:      chunking.ChunkTypeFile,
		Name:      filepath.Base(filename),
		Path:      filename,
		Content:   code,
		Language:  chunking.LanguageGo,
		StartLine: 1,
		EndLine:   len(lines),
		Metadata: map[string]interface{}{
			"chunking_method": "fallback",
		},
	}

	// Generate ID
	chunk.ID = generateChunkID(chunk)

	return []*chunking.CodeChunk{chunk}
}

// generateChunkID generates a unique ID for a chunk based on its content and metadata
func generateChunkID(chunk *chunking.CodeChunk) string {
	// Combine type, name, path, and line numbers for a unique identifier
	idString := string(chunk.Type) + ":" + chunk.Path + ":" + strconv.Itoa(chunk.StartLine) + "-" + strconv.Itoa(chunk.EndLine)

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(idString))
	return hex.EncodeToString(hash[:])
}
