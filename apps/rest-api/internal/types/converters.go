// Package types provides shared types for the REST API internal packages
package types

import (
	pkgrepo "github.com/S-Corkum/devops-mcp/pkg/repository"
)

// ConvertToPkgEmbedding converts an internal embedding to pkg repository embedding
func ConvertToPkgEmbedding(internal *Embedding) *pkgrepo.Embedding {
	if internal == nil {
		return nil
	}
	return &pkgrepo.Embedding{
		ID:           internal.ID,
		ContextID:    internal.ContextID,
		ContentIndex: internal.ContentIndex,
		Text:         internal.Text,
		Embedding:    internal.Embedding,
		ModelID:      internal.ModelID,
		// Metadata field not used in pkg.Embedding
	}
}

// ConvertToInternalEmbedding converts a pkg repository embedding to internal one
func ConvertToInternalEmbedding(pkgEmb *pkgrepo.Embedding) *Embedding {
	if pkgEmb == nil {
		return nil
	}
	return &Embedding{
		ID:           pkgEmb.ID,
		ContextID:    pkgEmb.ContextID,
		ContentIndex: pkgEmb.ContentIndex,
		Text:         pkgEmb.Text,
		Embedding:    pkgEmb.Embedding,
		ModelID:      pkgEmb.ModelID,
		// No Metadata mapping
	}
}

// ConvertToPkgEmbeddings converts a slice of internal embeddings to pkg repository embeddings
func ConvertToPkgEmbeddings(internalEmbs []*Embedding) []*pkgrepo.Embedding {
	if internalEmbs == nil {
		return nil
	}
	result := make([]*pkgrepo.Embedding, len(internalEmbs))
	for i, emb := range internalEmbs {
		result[i] = ConvertToPkgEmbedding(emb)
	}
	return result
}

// ConvertToInternalEmbeddings converts a slice of pkg repository embeddings to internal ones
func ConvertToInternalEmbeddings(pkgEmbs []*pkgrepo.Embedding) []*Embedding {
	if pkgEmbs == nil {
		return nil
	}
	result := make([]*Embedding, len(pkgEmbs))
	for i, emb := range pkgEmbs {
		result[i] = ConvertToInternalEmbedding(emb)
	}
	return result
}
