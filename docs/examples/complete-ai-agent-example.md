# Complete AI Agent Integration Example

This guide provides a complete, end-to-end example of how to integrate an AI agent with the MCP Server. We'll build a simple but functional DevOps assistant that can interact with GitHub.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Setting Up the Project](#setting-up-the-project)
- [Implementing the AI Agent](#implementing-the-ai-agent)
  - [Step 1: Client Setup](#step-1-client-setup)
  - [Step 2: Context Management](#step-2-context-management)
  - [Step 3: Tool Discovery](#step-3-tool-discovery)
  - [Step 4: User Message Processing](#step-4-user-message-processing)
  - [Step 5: Executing GitHub Actions](#step-5-executing-github-actions)
  - [Step 6: Semantic Search](#step-6-semantic-search)
- [Full Code Example](#full-code-example)
- [Running the Agent](#running-the-agent)
- [Further Enhancements](#further-enhancements)

## Overview

In this example, we'll create an AI agent that:

1. Integrates with an LLM (like OpenAI's GPT-4 or Anthropic's Claude)
2. Maintains conversation context using MCP Server
3. Discovers and uses GitHub tools via MCP Server
4. Implements semantic search using vector embeddings

The agent will help users:
- Create GitHub issues
- Search repositories
- Find pull requests
- Access other GitHub functionality

## Prerequisites

- MCP Server up and running (see [Quick Start Guide](../quick-start-guide.md))
- Python 3.8+ installed
- OpenAI API key (or another LLM provider)
- GitHub personal access token

## Setting Up the Project

First, let's create a new Python project:

```bash
mkdir devops-assistant
cd devops-assistant
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
```

Install required packages:

```bash
pip install requests openai python-dotenv
```

Create a `.env` file with your credentials:

```bash
# .env
OPENAI_API_KEY=your_openai_api_key
MCP_SERVER_URL=http://localhost:8080
MCP_API_KEY=your_mcp_api_key
GITHUB_TOKEN=your_github_token
```

## Implementing the AI Agent

Let's break down the implementation into manageable steps:

### Step 1: Client Setup

Create a file called `mcp_client.py` to handle communication with MCP Server:

```python
# mcp_client.py
import os
import requests
from dotenv import load_dotenv

load_dotenv()

class MCPClient:
    def __init__(self, base_url=None, api_key=None):
        self.base_url = base_url or os.getenv("MCP_SERVER_URL")
        self.api_key = api_key or os.getenv("MCP_API_KEY")
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}"
        }
    
    def create_context(self, context_data):
        response = requests.post(
            f"{self.base_url}/api/v1/contexts",
            headers=self.headers,
            json=context_data
        )
        response.raise_for_status()
        return response.json()
    
    def get_context(self, context_id):
        response = requests.get(
            f"{self.base_url}/api/v1/contexts/{context_id}",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json()
    
    def update_context(self, context_id, context_data, options=None):
        payload = {
            "context": context_data,
            "options": options or {}
        }
        response = requests.put(
            f"{self.base_url}/api/v1/contexts/{context_id}",
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()
    
    def list_tools(self):
        response = requests.get(
            f"{self.base_url}/api/v1/tools",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json().get("tools", [])
    
    def list_tool_actions(self, tool):
        response = requests.get(
            f"{self.base_url}/api/v1/tools/{tool}/actions",
            headers=self.headers
        )
        response.raise_for_status()
        return response.json().get("allowed_actions", [])
    
    def execute_tool_action(self, context_id, tool, action, params):
        response = requests.post(
            f"{self.base_url}/api/v1/tools/{tool}/actions/{action}?context_id={context_id}",
            headers=self.headers,
            json=params
        )
        response.raise_for_status()
        return response.json()
    
    def query_tool_data(self, tool, params):
        response = requests.post(
            f"{self.base_url}/api/v1/tools/{tool}/query",
            headers=self.headers,
            json=params
        )
        response.raise_for_status()
        return response.json()
    
    def store_embedding(self, embedding_data):
        response = requests.post(
            f"{self.base_url}/api/v1/vectors/store",
            headers=self.headers,
            json=embedding_data
        )
        response.raise_for_status()
        return response.json()
    
    def search_embeddings(self, search_data):
        response = requests.post(
            f"{self.base_url}/api/v1/vectors/search",
            headers=self.headers,
            json=search_data
        )
        response.raise_for_status()
        return response.json()
```

### Step 2: Context Management

Now, create a file for the AI agent implementation, `agent.py`:

```python
# agent.py
import os
import json
import openai
from dotenv import load_dotenv
from mcp_client import MCPClient

load_dotenv()

openai.api_key = os.getenv("OPENAI_API_KEY")
AGENT_ID = "devops-assistant"
MODEL_ID = "gpt-4"

class DevOpsAssistant:
    def __init__(self):
        self.mcp_client = MCPClient()
        self.context_id = None
        self.initialize_context()
    
    def initialize_context(self):
        # Create a system prompt that explains the agent's capabilities
        system_prompt = """
        You are a DevOps assistant that can help with GitHub operations.
        You can create issues, search repositories, find pull requests, and more.
        
        When a user asks for GitHub operations, use your built-in GitHub integration.
        Always respond in a helpful, concise manner.
        """
        
        # Calculate approximate token count (rough estimate)
        system_tokens = len(system_prompt.split()) * 1.3
        
        # Create the context with the system prompt
        context_data = {
            "agent_id": AGENT_ID,
            "model_id": MODEL_ID,
            "max_tokens": 4000,
            "content": [
                {
                    "role": "system",
                    "content": system_prompt,
                    "tokens": int(system_tokens)
                }
            ]
        }
        
        # Create the context
        result = self.mcp_client.create_context(context_data)
        self.context_id = result.get("id")
        print(f"Created context with ID: {self.context_id}")
    
    def get_current_context(self):
        if not self.context_id:
            self.initialize_context()
        
        return self.mcp_client.get_context(self.context_id)
    
    def add_user_message(self, message):
        # Add a user message to the context
        tokens = len(message.split()) * 1.3  # Rough estimate
        
        update_data = {
            "content": [
                {
                    "role": "user",
                    "content": message,
                    "tokens": int(tokens)
                }
            ]
        }
        
        options = {
            "truncate": True,
            "truncate_strategy": "oldest_first"
        }
        
        self.mcp_client.update_context(self.context_id, update_data, options)
    
    def add_assistant_message(self, message):
        # Add an assistant message to the context
        tokens = len(message.split()) * 1.3  # Rough estimate
        
        update_data = {
            "content": [
                {
                    "role": "assistant",
                    "content": message,
                    "tokens": int(tokens)
                }
            ]
        }
        
        options = {
            "truncate": True,
            "truncate_strategy": "oldest_first"
        }
        
        self.mcp_client.update_context(self.context_id, update_data, options)
```

### Step 3: Tool Discovery

Add methods to discover GitHub tools:

```python
# Add this to the DevOpsAssistant class in agent.py
def discover_github_capabilities(self):
    tools = self.mcp_client.list_tools()
    
    github_tool = next((tool for tool in tools if tool["name"] == "github"), None)
    if not github_tool:
        return "GitHub tool not available."
    
    actions = self.mcp_client.list_tool_actions("github")
    
    return {
        "description": github_tool.get("description", ""),
        "actions": actions
    }
```

### Step 4: User Message Processing

Add logic to process user messages using the OpenAI API:

```python
# Add this to the DevOpsAssistant class in agent.py
def process_user_message(self, user_message):
    # Add the user message to the context
    self.add_user_message(user_message)
    
    # Get the current context
    context = self.get_current_context()
    
    # Extract messages for the LLM
    messages = []
    for item in context.get("content", []):
        messages.append({
            "role": item["role"],
            "content": item["content"]
        })
    
    # Get a response from the LLM
    response = openai.ChatCompletion.create(
        model="gpt-4",
        messages=messages,
        temperature=0.7,
        max_tokens=500
    )
    
    assistant_message = response.choices[0].message["content"]
    
    # Add the assistant message to the context
    self.add_assistant_message(assistant_message)
    
    return assistant_message
```

### Step 5: Executing GitHub Actions

Now, add methods to execute GitHub actions:

```python
# Add this to the DevOpsAssistant class in agent.py
def create_github_issue(self, repo_owner, repo_name, title, body, labels=None):
    params = {
        "owner": repo_owner,
        "repo": repo_name,
        "title": title,
        "body": body
    }
    
    if labels:
        params["labels"] = labels
    
    result = self.mcp_client.execute_tool_action(
        self.context_id,
        "github",
        "create_issue",
        params
    )
    
    return result

def search_github_repos(self, query):
    params = {
        "query": query,
        "limit": 5
    }
    
    result = self.mcp_client.query_tool_data("github", params)
    
    return result
```

### Step 6: Semantic Search

Finally, add methods for vector embeddings and semantic search:

```python
# Add this to the DevOpsAssistant class in agent.py
def generate_embedding(self, text):
    response = openai.Embedding.create(
        model="text-embedding-ada-002",
        input=text
    )
    return response["data"][0]["embedding"]

def store_conversation_embedding(self, text, content_index):
    embedding = self.generate_embedding(text)
    
    embedding_data = {
        "context_id": self.context_id,
        "content_index": content_index,
        "text": text,
        "embedding": embedding,
        "model_id": "text-embedding-ada-002"
    }
    
    self.mcp_client.store_embedding(embedding_data)

def search_similar_content(self, query_text):
    query_embedding = self.generate_embedding(query_text)
    
    search_data = {
        "context_id": self.context_id,
        "query_embedding": query_embedding,
        "limit": 5,
        "model_id": "text-embedding-ada-002",
        "similarity_threshold": 0.7
    }
    
    results = self.mcp_client.search_embeddings(search_data)
    
    return results
```

## Full Code Example

Combine all the code above to create a complete AI agent implementation. Let's also create a simple CLI application to interact with the agent:

```python
# main.py
from agent import DevOpsAssistant

def main():
    print("DevOps Assistant Initialized")
    print("Type 'exit' to quit")
    
    agent = DevOpsAssistant()
    
    while True:
        user_input = input("\nYou: ")
        
        if user_input.lower() == 'exit':
            break
        
        response = agent.process_user_message(user_input)
        print(f"\nAssistant: {response}")
        
        # Store embedding for semantic search
        agent.store_conversation_embedding(user_input, 0)

if __name__ == "__main__":
    main()
```

## Running the Agent

To run the agent:

```bash
python main.py
```

Example conversation:

```
DevOps Assistant Initialized
Type 'exit' to quit

You: Can you help me create a GitHub issue for a login bug?