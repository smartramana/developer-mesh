# AI Agent Integration

This guide demonstrates how to integrate an AI agent (like an LLM-based assistant) with the DevOps MCP platform to enable operations on DevOps tools.

## Overview

AI agents can leverage DevOps MCP to:

1. Store and retrieve conversation context
2. Perform semantic searches on past conversations
3. Execute operations on DevOps tools (e.g., GitHub)
4. Receive notifications from tool webhooks

## Example: Building an AI Assistant with DevOps Capabilities

### 1. Setup Authentication

```python
import requests
import os

# Set up API key
API_KEY = os.environ.get("MCP_API_KEY", "your-api-key")

# Set base URL
MCP_BASE_URL = "http://localhost:8080/api/v1"
VECTOR_API_URL = "http://localhost:8081/api/v1"

# Create headers with authentication
headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}
```

### 2. Create a Context for User Conversation

```python
import json
import uuid

def create_conversation_context(user_id, initial_message):
    # Generate a unique context ID
    context_id = f"conversation-{uuid.uuid4()}"
    
    # Create initial context
    data = {
        "id": context_id,
        "user_id": user_id,
        "content": initial_message,
        "metadata": {
            "source": "user_conversation",
            "timestamp": datetime.now().isoformat()
        }
    }
    
    # Send request to create context
    response = requests.post(
        f"{MCP_BASE_URL}/mcp/context",
        headers=headers,
        data=json.dumps(data)
    )
    
    if response.status_code == 200:
        return response.json()["context_id"]
    else:
        print(f"Error creating context: {response.text}")
        return None
```

### 3. Store Vector Embeddings of Conversation

```python
import numpy as np
import openai

# Configure OpenAI API (or your preferred embedding provider)
openai.api_key = os.environ.get("OPENAI_API_KEY")

def get_embedding(text):
    """Generate a vector embedding for text using OpenAI"""
    response = openai.Embedding.create(
        model="text-embedding-ada-002",
        input=text
    )
    return response["data"][0]["embedding"]

def store_conversation_embedding(context_id, text, content_index):
    """Store a conversation chunk as a vector embedding"""
    # Generate embedding
    embedding = get_embedding(text)
    
    # Prepare data
    data = {
        "id": f"emb-{uuid.uuid4()}",
        "context_id": context_id,
        "content_index": content_index,
        "text": text,
        "embedding": embedding,
        "model_id": "text-embedding-ada-002",
        "metadata": {
            "type": "conversation",
            "timestamp": datetime.now().isoformat()
        }
    }
    
    # Send request
    response = requests.post(
        f"{VECTOR_API_URL}/vectors",
        headers=headers,
        data=json.dumps(data)
    )
    
    return response.status_code == 200
```

### 4. Search for Relevant Context

```python
def semantic_search(context_id, query_text, limit=5):
    """Search for semantically similar content in context"""
    # Generate embedding for the query
    query_embedding = get_embedding(query_text)
    
    # Prepare search request
    data = {
        "context_id": context_id,
        "model_id": "text-embedding-ada-002",
        "query_embedding": query_embedding,
        "limit": limit,
        "similarity_threshold": 0.7
    }
    
    # Send search request
    response = requests.post(
        f"{VECTOR_API_URL}/vectors/search",
        headers=headers,
        data=json.dumps(data)
    )
    
    if response.status_code == 200:
        results = response.json()["embeddings"]
        # Extract text and similarity from results
        relevant_content = [
            {
                "text": item["Text"],
                "similarity": item["Metadata"]["similarity"],
                "index": item["ContentIndex"]
            }
            for item in results
        ]
        return relevant_content
    else:
        print(f"Search error: {response.text}")
        return []
```

### 5. Execute GitHub Operations via MCP

```python
def create_github_issue(repo_owner, repo_name, title, body, labels=None):
    """Create a GitHub issue through DevOps MCP"""
    data = {
        "owner": repo_owner,
        "repo": repo_name,
        "title": title,
        "body": body,
        "labels": labels or []
    }
    
    response = requests.post(
        f"{MCP_BASE_URL}/tools/github/actions/create_issue",
        headers=headers,
        data=json.dumps(data)
    )
    
    if response.status_code == 200:
        return response.json()
    else:
        print(f"Error creating issue: {response.text}")
        return None
```

### 6. Putting It All Together

Here's how you can integrate this with an AI assistant:

```python
class DevOpsAssistant:
    def __init__(self, user_id):
        self.user_id = user_id
        self.context_id = None
        self.message_counter = 0
    
    def initialize_conversation(self, initial_message):
        self.context_id = create_conversation_context(self.user_id, initial_message)
        self.store_message("user", initial_message)
    
    def store_message(self, role, content):
        self.message_counter += 1
        return store_conversation_embedding(
            self.context_id,
            f"{role}: {content}",
            self.message_counter
        )
    
    def get_relevant_context(self, query):
        return semantic_search(self.context_id, query)
    
    def handle_user_message(self, message):
        # Store user message
        self.store_message("user", message)
        
        # Find relevant context
        relevant_context = self.get_relevant_context(message)
        
        # Generate AI response (using your preferred AI model)
        ai_response = generate_ai_response(message, relevant_context)
        
        # Store AI response
        self.store_message("assistant", ai_response)
        
        # Check if it's a GitHub request
        if "create issue" in message.lower():
            # Parse issue details from message or AI response
            # This is a simplified example
            issue_details = extract_issue_details(message, ai_response)
            
            # Create GitHub issue
            issue = create_github_issue(
                issue_details["owner"],
                issue_details["repo"],
                issue_details["title"],
                issue_details["body"]
            )
            
            if issue:
                return f"{ai_response}\n\nI've created the issue for you: {issue['html_url']}"
        
        return ai_response
```

### 7. Sample Usage

```python
# Initialize the assistant
assistant = DevOpsAssistant(user_id="user123")

# Start conversation
assistant.initialize_conversation("Hi, I need help with my GitHub project.")

# Handle user request
response = assistant.handle_user_message(
    "Can you create an issue in my repo S-Corkum/my-project to fix the login bug?"
)

print(response)
# Output: "I've created the issue for you: https://github.com/S-Corkum/my-project/issues/42"
```

## Integration with OpenAI Assistant API

For a more advanced integration using OpenAI's Assistant API:

```python
from openai import OpenAI
import json

client = OpenAI()

# Create an assistant with DevOps MCP function
assistant = client.beta.assistants.create(
    name="DevOps Assistant",
    instructions="You are a DevOps assistant that can help with GitHub operations.",
    model="gpt-4-turbo",
    tools=[{
        "type": "function",
        "function": {
            "name": "create_github_issue",
            "description": "Create a GitHub issue through DevOps MCP",
            "parameters": {
                "type": "object",
                "properties": {
                    "owner": {"type": "string"},
                    "repo": {"type": "string"},
                    "title": {"type": "string"},
                    "body": {"type": "string"},
                    "labels": {"type": "array", "items": {"type": "string"}}
                },
                "required": ["owner", "repo", "title", "body"]
            }
        }
    }]
)

# Create a thread
thread = client.beta.threads.create()

# Add a message to the thread
message = client.beta.threads.messages.create(
    thread_id=thread.id,
    role="user",
    content="Create an issue in S-Corkum/my-project called 'Fix login bug' with description 'Users cannot log in when using special characters in passwords'."
)

# Run the assistant
run = client.beta.threads.runs.create(
    thread_id=thread.id,
    assistant_id=assistant.id
)

# Wait for completion and process any tool calls
# This would need a proper polling mechanism
run = client.beta.threads.runs.retrieve(
    thread_id=thread.id,
    run_id=run.id
)

# If there are tool calls, execute them through MCP
if run.status == "requires_action" and run.required_action.type == "submit_tool_outputs":
    tool_calls = run.required_action.submit_tool_outputs.tool_calls
    tool_outputs = []
    
    for tool_call in tool_calls:
        if tool_call.function.name == "create_github_issue":
            # Parse arguments
            args = json.loads(tool_call.function.arguments)
            
            # Call DevOps MCP
            issue = create_github_issue(
                args["owner"],
                args["repo"],
                args["title"],
                args["body"],
                args.get("labels", [])
            )
            
            # Add tool output
            tool_outputs.append({
                "tool_call_id": tool_call.id,
                "output": json.dumps(issue)
            })
    
    # Submit tool outputs
    run = client.beta.threads.runs.submit_tool_outputs(
        thread_id=thread.id,
        run_id=run.id,
        tool_outputs=tool_outputs
    )
```

## Best Practices

1. **Store Context Efficiently**: Break conversations into logical chunks before storing
2. **Use Consistent Model IDs**: Keep track of which embedding model you use
3. **Handle Authentication Securely**: Store API keys securely and rotate regularly
4. **Implement Rate Limiting**: Add retry logic for API requests to handle rate limits
5. **Design Effective Prompts**: Include relevant context when sending to your LLM
