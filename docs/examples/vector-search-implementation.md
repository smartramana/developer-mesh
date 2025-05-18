# Vector Search Implementation

This guide demonstrates how to implement semantic vector search capabilities using DevOps MCP. You'll learn how to store vector embeddings and perform similarity searches to find semantically relevant content.

## Overview

Vector search allows you to find content based on semantic meaning rather than exact keyword matches. DevOps MCP uses PostgreSQL with pgvector extension to provide efficient vector search capabilities.

## Use Cases

- Searching documentation or knowledge bases
- Finding similar issues or pull requests
- Retrieving relevant conversation context for AI agents
- Implementing semantic search in DevOps tools

## Example Implementation

### 1. Setup Environment

```python
import requests
import numpy as np
import json
import os
import uuid
from datetime import datetime

# Configuration
API_KEY = os.environ.get("MCP_API_KEY", "your-api-key")
VECTOR_API_URL = "http://localhost:8081/api/v1"

headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}
```

### 2. Generate Embeddings

You can use various models to generate embeddings. Here's an example using OpenAI's embedding model:

```python
import openai

# Configure OpenAI API
openai.api_key = os.environ.get("OPENAI_API_KEY")

def generate_embedding(text, model="text-embedding-ada-002"):
    """Generate a vector embedding for text"""
    response = openai.Embedding.create(
        model=model,
        input=text
    )
    return response["data"][0]["embedding"]
```

### 3. Store Documents as Embeddings

```python
def store_document_embedding(context_id, document, model_id="text-embedding-ada-002"):
    """Store a document as a vector embedding"""
    # Generate a unique ID for the embedding
    embedding_id = f"doc-{uuid.uuid4()}"
    
    # Generate the embedding vector
    embedding_vector = generate_embedding(document["text"], model_id)
    
    # Prepare the request data
    data = {
        "id": embedding_id,
        "context_id": context_id,
        "content_index": document.get("index", 1),
        "text": document["text"],
        "embedding": embedding_vector,
        "model_id": model_id,
        "metadata": {
            **document.get("metadata", {}),  # Include any additional metadata
            "source": document.get("source", "unknown"),
            "timestamp": datetime.now().isoformat()
        }
    }
    
    # Send request to store the embedding
    response = requests.post(
        f"{VECTOR_API_URL}/vectors",
        headers=headers,
        data=json.dumps(data)
    )
    
    if response.status_code == 200:
        print(f"Successfully stored embedding {embedding_id}")
        return embedding_id
    else:
        print(f"Error storing embedding: {response.text}")
        return None
```

### 4. Batch Process Documents

For efficiency, you can process multiple documents in a batch:

```python
def batch_process_documents(context_id, documents, model_id="text-embedding-ada-002"):
    """Process a batch of documents and store their embeddings"""
    embedding_ids = []
    
    for index, document in enumerate(documents):
        # Include the index in the document
        document["index"] = index + 1
        
        # Store the embedding
        embedding_id = store_document_embedding(
            context_id,
            document,
            model_id
        )
        
        if embedding_id:
            embedding_ids.append(embedding_id)
    
    print(f"Processed {len(embedding_ids)} documents")
    return embedding_ids
```

### 5. Perform Semantic Search

```python
def semantic_search(context_id, query_text, model_id="text-embedding-ada-002", limit=5, threshold=0.7):
    """Search for semantically similar content"""
    # Generate embedding for the query
    query_embedding = generate_embedding(query_text, model_id)
    
    # Prepare search request
    data = {
        "context_id": context_id,
        "model_id": model_id,
        "query_embedding": query_embedding,
        "limit": limit,
        "similarity_threshold": threshold
    }
    
    # Send search request
    response = requests.post(
        f"{VECTOR_API_URL}/vectors/search",
        headers=headers,
        data=json.dumps(data)
    )
    
    if response.status_code == 200:
        results = response.json()["embeddings"]
        
        # Process and format results
        search_results = []
        for item in results:
            search_results.append({
                "id": item["ID"],
                "text": item["Text"],
                "similarity": item["Metadata"].get("similarity", 0),
                "source": item["Metadata"].get("source", "unknown"),
                "content_index": item["ContentIndex"],
                "model_id": item["ModelID"]
            })
        
        return search_results
    else:
        print(f"Search error: {response.text}")
        return []
```

### 6. Complete Working Example: Documentation Search System

Here's a complete example implementing a documentation search system:

```python
class DocumentationSearchSystem:
    def __init__(self, context_id=None):
        self.context_id = context_id or f"docs-{uuid.uuid4()}"
        self.embedding_model = "text-embedding-ada-002"
    
    def ingest_documentation(self, documents):
        """Ingest multiple documentation files"""
        return batch_process_documents(
            self.context_id,
            documents,
            self.embedding_model
        )
    
    def search_documentation(self, query, limit=5):
        """Search through documentation"""
        return semantic_search(
            self.context_id,
            query,
            self.embedding_model,
            limit,
            0.7
        )

# Example usage
if __name__ == "__main__":
    # Sample documentation files
    documents = [
        {
            "text": "The adapter pattern is a structural design pattern that allows objects with incompatible interfaces to collaborate. It wraps an instance of one class into an adapter that implements the interface another class expects.",
            "source": "design_patterns.md",
            "metadata": {"section": "Structural Patterns", "author": "Gamma et al."}
        },
        {
            "text": "Go workspaces allow you to work with multiple modules in a single working directory. This is useful for developers who need to make coordinated changes across multiple modules.",
            "source": "go_concepts.md",
            "metadata": {"section": "Go Modules", "author": "Go Team"}
        },
        {
            "text": "PostgreSQL with pgvector extension provides efficient vector similarity search capabilities. It supports multiple distance metrics including cosine distance, Euclidean distance, and dot product.",
            "source": "databases.md",
            "metadata": {"section": "Vector Databases", "author": "Database Team"}
        }
    ]
    
    # Initialize the search system
    search_system = DocumentationSearchSystem()
    
    # Ingest documents
    search_system.ingest_documentation(documents)
    
    # Perform a search
    results = search_system.search_documentation("How do Go modules work together?")
    
    # Display results
    for i, result in enumerate(results, 1):
        print(f"{i}. {result['text']} (Similarity: {result['similarity']:.4f})")
        print(f"   Source: {result['source']}")
        print()
```

### 7. Real-world Example: Technical Knowledge Base

Here's a more complex example of building a technical knowledge base search:

```python
import re
import os

class TechnicalKnowledgeBase:
    def __init__(self, kb_id=None):
        self.kb_id = kb_id or f"kb-{uuid.uuid4()}"
        self.embedding_model = "text-embedding-ada-002"
    
    def chunk_document(self, doc_text, chunk_size=1000, overlap=200):
        """Split document into overlapping chunks"""
        chunks = []
        
        # Simple chunking strategy
        doc_length = len(doc_text)
        start = 0
        
        while start < doc_length:
            end = min(start + chunk_size, doc_length)
            
            # Try to find a sentence boundary
            if end < doc_length:
                # Look for sentence end within 200 chars of the chunking point
                search_end = min(end + 200, doc_length)
                sentence_end = -1
                
                for match in re.finditer(r'[.!?]\s+', doc_text[end:search_end]):
                    sentence_end = end + match.end()
                    break
                
                if sentence_end > 0:
                    end = sentence_end
            
            chunks.append(doc_text[start:end])
            
            # Move start position, with overlap
            start = end - overlap if end < doc_length else doc_length
        
        return chunks
    
    def process_markdown_file(self, filepath, source_name=None):
        """Process a Markdown file, chunking it and storing embeddings"""
        if not os.path.exists(filepath):
            print(f"File not found: {filepath}")
            return []
        
        # Use filename as source if not provided
        source = source_name or os.path.basename(filepath)
        
        with open(filepath, 'r') as f:
            content = f.read()
        
        # Extract title from first heading
        title_match = re.search(r'^# (.*?)$', content, re.MULTILINE)
        title = title_match.group(1) if title_match else source
        
        # Chunk the document
        chunks = self.chunk_document(content)
        
        # Prepare documents for embedding
        documents = []
        for idx, chunk in enumerate(chunks):
            documents.append({
                "text": chunk,
                "source": source,
                "metadata": {
                    "title": title,
                    "chunk": idx + 1,
                    "total_chunks": len(chunks)
                }
            })
        
        # Store embeddings
        return batch_process_documents(self.kb_id, documents, self.embedding_model)
    
    def bulk_process_directory(self, directory_path):
        """Process all markdown files in a directory"""
        if not os.path.isdir(directory_path):
            print(f"Directory not found: {directory_path}")
            return
        
        all_embeddings = []
        
        for root, _, files in os.walk(directory_path):
            for file in files:
                if file.endswith('.md'):
                    filepath = os.path.join(root, file)
                    relative_path = os.path.relpath(filepath, directory_path)
                    
                    print(f"Processing {relative_path}...")
                    embeddings = self.process_markdown_file(filepath, relative_path)
                    all_embeddings.extend(embeddings)
        
        print(f"Processed {len(all_embeddings)} embeddings from markdown files")
        return all_embeddings
    
    def search(self, query, limit=5):
        """Search the knowledge base"""
        results = semantic_search(
            self.kb_id,
            query,
            self.embedding_model,
            limit,
            0.7
        )
        
        return results
```

### Example Usage

```python
# Initialize the knowledge base
kb = TechnicalKnowledgeBase()

# Process documentation directory
kb.bulk_process_directory("/path/to/your/docs")

# Search the knowledge base
results = kb.search("How do I implement the adapter pattern in Go?")

# Display results with highlighted matches
for idx, result in enumerate(results, 1):
    print(f"{idx}. [{result['source']}] (Similarity: {result['similarity']:.4f})")
    print(f"   {result['text'][:200]}...")
    print()
```

## Performance Considerations

1. **Vector Dimensions**: The system supports various vector dimensions (typically 768-1536 dimensions), but all vectors for the same model ID should have consistent dimensions.

2. **Batch Processing**: For large document sets, process documents in batches to avoid overwhelming the API.

3. **Chunking Strategy**: The chunking approach affects search quality. Consider:
   - Semantic boundaries (paragraphs, sections)
   - Overlap between chunks (typically 10-20%)
   - Chunk size (typically 500-1000 tokens)

4. **Metadata Usage**: Store relevant metadata with embeddings to improve filtering and result organization.

5. **Model Selection**: Different embedding models have different characteristics:
   - OpenAI text-embedding-ada-002 (1536 dimensions)
   - Cohere embed-english-v3.0 (1024 dimensions)
   - Open-source models like all-MiniLM-L6-v2 (384 dimensions)

## Similarity Score Interpretation

The similarity score is included in the `Metadata` field of each embedding in search results:

- 0.95-1.0: Nearly identical semantic meaning
- 0.9-0.95: Very similar meaning
- 0.8-0.9: Similar meaning/topic
- 0.7-0.8: Related topics
- Below 0.7: Generally unrelated

## Example Applications

1. **Documentation Search**: Build a semantic search system for technical documentation
2. **Issue Similarity**: Find similar GitHub issues based on description
3. **Knowledge Retrieval**: Implement RAG (Retrieval Augmented Generation) for AI assistants
4. **Content Clustering**: Group related content based on semantic similarity
