package api

import (
	"fmt"
	"strings"
)

// GraphQLBuilder provides a builder for GraphQL queries
type GraphQLBuilder struct {
	operation     string
	operationType string
	fields        []string
	variables     map[string]string
	arguments     map[string]string
	fragments     map[string]string
	includes      map[string][]string
	aliases       map[string]string
	parent        *GraphQLBuilder
	children      map[string]*GraphQLBuilder
}

// NewGraphQLBuilder creates a new GraphQL query builder
func NewGraphQLBuilder(operationType, operation string) *GraphQLBuilder {
	return &GraphQLBuilder{
		operation:     operation,
		operationType: operationType,
		fields:        make([]string, 0),
		variables:     make(map[string]string),
		arguments:     make(map[string]string),
		fragments:     make(map[string]string),
		includes:      make(map[string][]string),
		aliases:       make(map[string]string),
		children:      make(map[string]*GraphQLBuilder),
	}
}

// NewQuery creates a new query builder
func NewQuery(operation string) *GraphQLBuilder {
	return NewGraphQLBuilder("query", operation)
}

// NewMutation creates a new mutation builder
func NewMutation(operation string) *GraphQLBuilder {
	return NewGraphQLBuilder("mutation", operation)
}

// AddField adds a field to the query
func (b *GraphQLBuilder) AddField(field string) *GraphQLBuilder {
	b.fields = append(b.fields, field)
	return b
}

// AddFields adds multiple fields to the query
func (b *GraphQLBuilder) AddFields(fields ...string) *GraphQLBuilder {
	b.fields = append(b.fields, fields...)
	return b
}

// AddVariable adds a variable to the query
func (b *GraphQLBuilder) AddVariable(name, type_ string) *GraphQLBuilder {
	b.variables[name] = type_
	return b
}

// AddArgument adds an argument to the query
func (b *GraphQLBuilder) AddArgument(name, value string) *GraphQLBuilder {
	b.arguments[name] = value
	return b
}

// AddFragment adds a fragment to the query
func (b *GraphQLBuilder) AddFragment(name, type_, fields string) *GraphQLBuilder {
	b.fragments[name] = fmt.Sprintf("fragment %s on %s { %s }", name, type_, fields)
	return b
}

// Include includes a fragment in a field
func (b *GraphQLBuilder) Include(field, fragment string) *GraphQLBuilder {
	if _, ok := b.includes[field]; !ok {
		b.includes[field] = make([]string, 0)
	}
	b.includes[field] = append(b.includes[field], fragment)
	return b
}

// AddAlias adds an alias for a field
func (b *GraphQLBuilder) AddAlias(field, alias string) *GraphQLBuilder {
	b.aliases[field] = alias
	return b
}

// AddChild adds a child builder for a nested object
func (b *GraphQLBuilder) AddChild(field string) *GraphQLBuilder {
	child := &GraphQLBuilder{
		operation:     field,
		operationType: "",
		fields:        make([]string, 0),
		variables:     make(map[string]string),
		arguments:     make(map[string]string),
		fragments:     make(map[string]string),
		includes:      make(map[string][]string),
		aliases:       make(map[string]string),
		parent:        b,
		children:      make(map[string]*GraphQLBuilder),
	}
	b.children[field] = child
	return child
}

// End returns to the parent builder
func (b *GraphQLBuilder) End() *GraphQLBuilder {
	if b.parent == nil {
		return b
	}
	return b.parent
}

// String returns the GraphQL query as a string
func (b *GraphQLBuilder) String() string {
	var sb strings.Builder

	// Start operation
	sb.WriteString(b.operationType)

	// Add operation name
	if b.operation != "" {
		sb.WriteString(" ")
		sb.WriteString(b.operation)
	}

	// Add variables
	if len(b.variables) > 0 {
		sb.WriteString("(")
		first := true
		for name, type_ := range b.variables {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString("$")
			sb.WriteString(name)
			sb.WriteString(": ")
			sb.WriteString(type_)
			first = false
		}
		sb.WriteString(")")
	}

	// Start selection set
	sb.WriteString(" {\n")

	// Add operation arguments
	if len(b.arguments) > 0 && b.operation != "" {
		sb.WriteString("  ")
		sb.WriteString(b.operation)
		sb.WriteString("(")
		first := true
		for name, value := range b.arguments {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(name)
			sb.WriteString(": ")
			sb.WriteString(value)
			first = false
		}
		sb.WriteString(")")
		sb.WriteString(" {\n")
	}

	// Add fields
	for _, field := range b.fields {
		sb.WriteString("  ")

		// Add indentation if we have operation arguments
		if len(b.arguments) > 0 && b.operation != "" {
			sb.WriteString("  ")
		}

		// Add alias if available
		if alias, ok := b.aliases[field]; ok {
			sb.WriteString(alias)
			sb.WriteString(": ")
		}

		sb.WriteString(field)

		// Add fragment includes
		if includes, ok := b.includes[field]; ok && len(includes) > 0 {
			sb.WriteString(" {\n    ...") // Extra indentation for fragment
			sb.WriteString(strings.Join(includes, "\n    ..."))
			sb.WriteString("\n  }")
		}

		sb.WriteString("\n")
	}

	// Add children
	for field, child := range b.children {
		sb.WriteString("  ")

		// Add indentation if we have operation arguments
		if len(b.arguments) > 0 && b.operation != "" {
			sb.WriteString("  ")
		}

		// Add alias if available
		if alias, ok := b.aliases[field]; ok {
			sb.WriteString(alias)
			sb.WriteString(": ")
		}

		sb.WriteString(field)
		sb.WriteString(" {\n")

		// Add child fields
		childContent := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(child.String(), child.operationType+" "+child.operation+" {\n"), "\n}"), "\n", "\n  ")
		sb.WriteString("  ")
		sb.WriteString(childContent)
		sb.WriteString("\n  }")
		sb.WriteString("\n")
	}

	// Close operation arguments block
	if len(b.arguments) > 0 && b.operation != "" {
		sb.WriteString("  }\n")
	}

	// Close selection set
	sb.WriteString("}")

	// Add fragments
	if len(b.fragments) > 0 {
		sb.WriteString("\n\n")
		for _, fragment := range b.fragments {
			sb.WriteString(fragment)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// Build returns the GraphQL query and variables
func (b *GraphQLBuilder) Build() (string, map[string]any) {
	query := b.String()
	variables := make(map[string]any)

	// Convert string variables to any for the client
	// In real use, variables would be populated with actual values

	return query, variables
}

// Common Repository Queries

// BuildRepositoryQuery builds a query to get a repository
func BuildRepositoryQuery(owner, name string, withIssues, withPullRequests bool) *GraphQLBuilder {
	builder := NewQuery("GetRepository")
	builder.AddVariable("owner", "String!")
	builder.AddVariable("name", "String!")

	repository := builder.AddChild("repository")
	repository.AddArgument("owner", "$owner")
	repository.AddArgument("name", "$name")

	repository.AddFields(
		"id",
		"name",
		"owner { login }",
		"description",
		"url",
		"homepageUrl",
		"primaryLanguage { name }",
		"isPrivate",
		"stargazerCount",
		"forkCount",
		"createdAt",
		"updatedAt",
	)

	if withIssues {
		issues := repository.AddChild("issues")
		issues.AddArgument("first", "10")
		issues.AddArgument("states", "[OPEN]")
		issues.AddFields(
			"totalCount",
			"nodes { id, number, title, url, state, createdAt, author { login } }",
		)
	}

	if withPullRequests {
		prs := repository.AddChild("pullRequests")
		prs.AddArgument("first", "10")
		prs.AddArgument("states", "[OPEN]")
		prs.AddFields(
			"totalCount",
			"nodes { id, number, title, url, state, createdAt, author { login } }",
		)
	}

	return builder
}

// BuildUserRepositoriesQuery builds a query to list repositories for a user
func BuildUserRepositoriesQuery(login string, first int) *GraphQLBuilder {
	builder := NewQuery("GetUserRepositories")
	builder.AddVariable("login", "String!")
	builder.AddVariable("first", "Int!")

	user := builder.AddChild("user")
	user.AddArgument("login", "$login")

	repositories := user.AddChild("repositories")
	repositories.AddArgument("first", "$first")
	repositories.AddArgument("orderBy", "{field: UPDATED_AT, direction: DESC}")

	repositories.AddFields(
		"totalCount",
	)

	nodes := repositories.AddChild("nodes")
	nodes.AddFields(
		"id",
		"name",
		"owner { login }",
		"description",
		"url",
		"isPrivate",
		"stargazerCount",
		"forkCount",
		"createdAt",
		"updatedAt",
	)

	return builder
}

// Common Issue Queries

// BuildIssueQuery builds a query to get an issue
func BuildIssueQuery(owner, name string, number int) *GraphQLBuilder {
	builder := NewQuery("GetIssue")
	builder.AddVariable("owner", "String!")
	builder.AddVariable("name", "String!")
	builder.AddVariable("number", "Int!")

	repository := builder.AddChild("repository")
	repository.AddArgument("owner", "$owner")
	repository.AddArgument("name", "$name")

	issue := repository.AddChild("issue")
	issue.AddArgument("number", "$number")

	issue.AddFields(
		"id",
		"number",
		"title",
		"body",
		"state",
		"url",
		"createdAt",
		"updatedAt",
		"author { login }",
		"assignees(first: 10) { nodes { login } }",
		"labels(first: 10) { nodes { name } }",
	)

	comments := issue.AddChild("comments")
	comments.AddArgument("first", "10")
	comments.AddFields(
		"totalCount",
		"nodes { id, author { login }, body, createdAt }",
	)

	return builder
}

// BuildIssuesQuery builds a query to list issues
func BuildIssuesQuery(owner, name string, first int, states []string) *GraphQLBuilder {
	builder := NewQuery("GetIssues")
	builder.AddVariable("owner", "String!")
	builder.AddVariable("name", "String!")
	builder.AddVariable("first", "Int!")
	builder.AddVariable("states", "[IssueState!]")

	repository := builder.AddChild("repository")
	repository.AddArgument("owner", "$owner")
	repository.AddArgument("name", "$name")

	issues := repository.AddChild("issues")
	issues.AddArgument("first", "$first")
	issues.AddArgument("states", "$states")
	issues.AddArgument("orderBy", "{field: CREATED_AT, direction: DESC}")

	issues.AddFields(
		"totalCount",
	)

	nodes := issues.AddChild("nodes")
	nodes.AddFields(
		"id",
		"number",
		"title",
		"url",
		"state",
		"createdAt",
		"updatedAt",
		"author { login }",
		"assignees(first: 3) { nodes { login } }",
		"labels(first: 5) { nodes { name } }",
	)

	// Add page info for pagination
	pageInfo := issues.AddChild("pageInfo")
	pageInfo.AddFields(
		"hasNextPage",
		"endCursor",
	)

	return builder
}

// Common Pull Request Queries

// BuildPullRequestQuery builds a query to get a pull request
func BuildPullRequestQuery(owner, name string, number int) *GraphQLBuilder {
	builder := NewQuery("GetPullRequest")
	builder.AddVariable("owner", "String!")
	builder.AddVariable("name", "String!")
	builder.AddVariable("number", "Int!")

	repository := builder.AddChild("repository")
	repository.AddArgument("owner", "$owner")
	repository.AddArgument("name", "$name")

	pr := repository.AddChild("pullRequest")
	pr.AddArgument("number", "$number")

	pr.AddFields(
		"id",
		"number",
		"title",
		"body",
		"state",
		"url",
		"createdAt",
		"updatedAt",
		"author { login }",
		"assignees(first: 10) { nodes { login } }",
		"labels(first: 10) { nodes { name } }",
		"baseRefName",
		"headRefName",
		"mergeable",
		"merged",
		"mergedAt",
		"mergedBy { login }",
		"isDraft",
	)

	// Add commits
	commits := pr.AddChild("commits")
	commits.AddArgument("first", "10")
	commits.AddFields(
		"totalCount",
	)

	commitNodes := commits.AddChild("nodes")
	commitNodes.AddFields(
		"commit { oid, message, committedDate, author { name, email } }",
	)

	// Add reviews
	reviews := pr.AddChild("reviews")
	reviews.AddArgument("first", "10")
	reviews.AddFields(
		"totalCount",
		"nodes { id, author { login }, state, body, submittedAt }",
	)

	return builder
}

// Common Workflow Queries

// BuildWorkflowRunsQuery builds a query to list workflow runs
func BuildWorkflowRunsQuery(owner, name string, first int) *GraphQLBuilder {
	builder := NewQuery("GetWorkflowRuns")
	builder.AddVariable("owner", "String!")
	builder.AddVariable("name", "String!")
	builder.AddVariable("first", "Int!")

	repository := builder.AddChild("repository")
	repository.AddArgument("owner", "$owner")
	repository.AddArgument("name", "$name")

	// Note: This is a simplified example, the actual GitHub GraphQL schema
	// for workflows and actions might differ
	workflows := repository.AddChild("workflows")
	workflows.AddArgument("first", "$first")

	workflows.AddFields(
		"totalCount",
	)

	nodes := workflows.AddChild("nodes")
	nodes.AddFields(
		"id",
		"name",
		"state",
		"createdAt",
		"updatedAt",
	)

	// In real usage, you would consult the GitHub GraphQL schema explorer
	// to ensure your queries match the actual schema

	return builder
}

// Common User Queries

// BuildUserQuery builds a query to get a user
func BuildUserQuery(login string) *GraphQLBuilder {
	builder := NewQuery("GetUser")
	builder.AddVariable("login", "String!")

	user := builder.AddChild("user")
	user.AddArgument("login", "$login")

	user.AddFields(
		"id",
		"login",
		"name",
		"email",
		"bio",
		"company",
		"location",
		"websiteUrl",
		"avatarUrl",
		"createdAt",
		"updatedAt",
	)

	return builder
}

// Example Mutations

// BuildCreateIssueMutation builds a mutation to create an issue
func BuildCreateIssueMutation() *GraphQLBuilder {
	builder := NewMutation("CreateIssue")
	builder.AddVariable("repositoryId", "ID!")
	builder.AddVariable("title", "String!")
	builder.AddVariable("body", "String!")

	createIssue := builder.AddChild("createIssue")
	createIssue.AddArgument("input", "{repositoryId: $repositoryId, title: $title, body: $body}")

	issue := createIssue.AddChild("issue")
	issue.AddFields(
		"id",
		"number",
		"title",
		"url",
	)

	return builder
}

// BuildUpdateIssueMutation builds a mutation to update an issue
func BuildUpdateIssueMutation() *GraphQLBuilder {
	builder := NewMutation("UpdateIssue")
	builder.AddVariable("id", "ID!")
	builder.AddVariable("title", "String")
	builder.AddVariable("body", "String")
	builder.AddVariable("state", "IssueState")

	updateIssue := builder.AddChild("updateIssue")
	updateIssue.AddArgument("input", "{id: $id, title: $title, body: $body, state: $state}")

	issue := updateIssue.AddChild("issue")
	issue.AddFields(
		"id",
		"number",
		"title",
		"body",
		"state",
		"url",
	)

	return builder
}

// BuildAddCommentMutation builds a mutation to add a comment
func BuildAddCommentMutation() *GraphQLBuilder {
	builder := NewMutation("AddComment")
	builder.AddVariable("subjectId", "ID!")
	builder.AddVariable("body", "String!")

	addComment := builder.AddChild("addComment")
	addComment.AddArgument("input", "{subjectId: $subjectId, body: $body}")

	comment := addComment.AddChild("commentEdge")
	comment.AddFields(
		"node { id, body, url }",
	)

	return builder
}
