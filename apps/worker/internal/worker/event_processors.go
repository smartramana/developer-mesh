package worker

import (
	"context"
	"fmt"
	"strings"

	"github.com/S-Corkum/devops-mcp/apps/worker/internal/queue"
	"github.com/S-Corkum/devops-mcp/pkg/observability"
)

// BaseProcessor provides common functionality for all event processors
type BaseProcessor struct {
	Logger  observability.Logger
	Metrics observability.MetricsClient
}

// PushProcessor handles 'push' webhook events
type PushProcessor struct {
	BaseProcessor
}

// Process implements processing for push events
func (p *PushProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	p.Logger.Info("Processing push event", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"repo":        event.RepoName,
	})

	// Extract the ref (branch or tag that was pushed to)
	ref, ok := payload["ref"].(string)
	if !ok {
		return fmt.Errorf("invalid push event: missing or invalid ref")
	}

	// Extract head commit
	headCommit, ok := payload["head_commit"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid push event: missing or invalid head_commit")
	}

	// Log the push details
	p.Logger.Info("Push event details", map[string]interface{}{
		"ref":            ref,
		"head_commit_id": getMapStringValue(headCommit, "id"),
		"author":         getMapStringValue(getNestedMap(headCommit, "author"), "name"),
		"message":        getMapStringValue(headCommit, "message"),
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_push_events_processed", 1, map[string]string{
		"repo": event.RepoName,
		"ref":  ref,
	})

	branch := strings.TrimPrefix(ref, "refs/heads/")
	if strings.HasPrefix(ref, "refs/tags/") {
		// Handle tag push
		tag := strings.TrimPrefix(ref, "refs/tags/")
		p.Logger.Info("Tag push detected", map[string]interface{}{
			"tag": tag,
		})
		p.Metrics.IncrementCounter("github_tag_push_events", 1, map[string]string{
			"repo": event.RepoName,
		})
	} else {
		// Handle branch push
		p.Logger.Info("Branch push detected", map[string]interface{}{
			"branch": branch,
		})
		p.Metrics.IncrementCounter("github_branch_push_events", 1, map[string]string{
			"repo":   event.RepoName,
			"branch": branch,
		})
	}

	// Example: Implement real business logic here
	// - Update deployment tracking database
	// - Trigger CI/CD pipelines
	// - Update metrics/alerting systems
	// - Process dependencies
	
	return nil
}

// PullRequestProcessor handles 'pull_request' webhook events
type PullRequestProcessor struct {
	BaseProcessor
}

// Process implements processing for pull request events
func (p *PullRequestProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid pull request event: missing or invalid action")
	}

	// Extract the PR details
	pr, ok := payload["pull_request"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid pull request event: missing or invalid pull_request object")
	}

	prNumber, ok := pr["number"].(float64)
	if !ok {
		return fmt.Errorf("invalid pull request event: missing or invalid PR number")
	}

	prState := getMapStringValue(pr, "state")
	prTitle := getMapStringValue(pr, "title")
	
	p.Logger.Info("Processing pull request event", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"repo":        event.RepoName,
		"action":      action,
		"pr_number":   prNumber,
		"pr_title":    prTitle,
		"pr_state":    prState,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_pull_request_events_processed", 1, map[string]string{
		"repo":   event.RepoName,
		"action": action,
	})

	// Different logic based on the PR action
	switch action {
	case "opened":
		// Handle new PR
		p.handlePullRequestOpened(event, pr)
	case "closed":
		// Check if it was merged
		merged, ok := pr["merged"].(bool)
		if ok && merged {
			p.handlePullRequestMerged(event, pr)
		} else {
			p.handlePullRequestClosed(event, pr)
		}
	case "synchronize":
		// Handle PR updates (new commits pushed)
		p.handlePullRequestSynchronized(event, pr)
	case "reopened":
		// Handle PR reopened
		p.handlePullRequestReopened(event, pr)
	case "labeled", "unlabeled":
		// Handle label changes
		p.handlePullRequestLabels(event, pr, action)
	case "assigned", "unassigned":
		// Handle assignment changes
		p.handlePullRequestAssignment(event, pr, action)
	case "review_requested", "review_request_removed":
		// Handle review request changes
		p.handlePullRequestReviewRequest(event, pr, action)
	}

	return nil
}

// Helper methods for PullRequestProcessor
func (p *PullRequestProcessor) handlePullRequestOpened(event queue.SQSEvent, pr map[string]interface{}) {
	prNumber, _ := pr["number"].(float64)
	p.Logger.Info("New pull request opened", map[string]interface{}{
		"repo":      event.RepoName,
		"pr_number": prNumber,
		"title":     getMapStringValue(pr, "title"),
		"user":      getMapStringValue(getNestedMap(pr, "user"), "login"),
	})
}

func (p *PullRequestProcessor) handlePullRequestMerged(event queue.SQSEvent, pr map[string]interface{}) {
	prNumber, _ := pr["number"].(float64)
	p.Logger.Info("Pull request merged", map[string]interface{}{
		"repo":       event.RepoName,
		"pr_number":  prNumber,
		"merged_by":  getMapStringValue(getNestedMap(pr, "merged_by"), "login"),
		"merged_at":  getMapStringValue(pr, "merged_at"),
		"base":       getMapStringValue(getNestedMap(pr, "base"), "ref"),
		"head":       getMapStringValue(getNestedMap(pr, "head"), "ref"),
	})
}

func (p *PullRequestProcessor) handlePullRequestClosed(event queue.SQSEvent, pr map[string]interface{}) {
	prNumber, _ := pr["number"].(float64)
	p.Logger.Info("Pull request closed without merging", map[string]interface{}{
		"repo":      event.RepoName,
		"pr_number": prNumber,
		"closed_by": event.SenderName,
	})
}

func (p *PullRequestProcessor) handlePullRequestSynchronized(event queue.SQSEvent, pr map[string]interface{}) {
	prNumber, _ := pr["number"].(float64)
	p.Logger.Info("Pull request updated with new commits", map[string]interface{}{
		"repo":      event.RepoName,
		"pr_number": prNumber,
	})
}

func (p *PullRequestProcessor) handlePullRequestReopened(event queue.SQSEvent, pr map[string]interface{}) {
	prNumber, _ := pr["number"].(float64)
	p.Logger.Info("Pull request reopened", map[string]interface{}{
		"repo":      event.RepoName,
		"pr_number": prNumber,
	})
}

func (p *PullRequestProcessor) handlePullRequestLabels(event queue.SQSEvent, pr map[string]interface{}, action string) {
	prNumber, _ := pr["number"].(float64)
	var labelInfo map[string]interface{}
	// Note: In a real implementation, we would need the full payload
	// For now, just log what we have from the PR object
	
	p.Logger.Info("Pull request label changed", map[string]interface{}{
		"repo":       event.RepoName,
		"pr_number":  prNumber,
		"action":     action,
		"label_name": getMapStringValue(labelInfo, "name"),
	})
}

func (p *PullRequestProcessor) handlePullRequestAssignment(event queue.SQSEvent, pr map[string]interface{}, action string) {
	prNumber, _ := pr["number"].(float64)
	var assigneeInfo map[string]interface{}
	// Note: In a real implementation, we would need the full payload
	// For now, just use data from the PR object
	
	p.Logger.Info("Pull request assignment changed", map[string]interface{}{
		"repo":      event.RepoName,
		"pr_number": prNumber,
		"action":    action,
		"assignee":  getMapStringValue(assigneeInfo, "login"),
	})
}

func (p *PullRequestProcessor) handlePullRequestReviewRequest(event queue.SQSEvent, pr map[string]interface{}, action string) {
	prNumber, _ := pr["number"].(float64)
	var reviewerInfo map[string]interface{}
	// Note: In a real implementation, we would need the full payload
	// For now, just use data from the PR object
	
	p.Logger.Info("Pull request review request changed", map[string]interface{}{
		"repo":      event.RepoName,
		"pr_number": prNumber,
		"action":    action,
		"reviewer":  getMapStringValue(reviewerInfo, "login"),
	})
}

// IssuesProcessor handles 'issues' webhook events
type IssuesProcessor struct {
	BaseProcessor
}

// Process implements processing for issues events
func (p *IssuesProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid issues event: missing or invalid action")
	}

	// Extract the issue details
	issue, ok := payload["issue"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid issues event: missing or invalid issue object")
	}

	issueNumber, ok := issue["number"].(float64)
	if !ok {
		return fmt.Errorf("invalid issues event: missing or invalid issue number")
	}

	issueTitle := getMapStringValue(issue, "title")
	issueState := getMapStringValue(issue, "state")
	
	p.Logger.Info("Processing issue event", map[string]interface{}{
		"delivery_id":  event.DeliveryID,
		"repo":         event.RepoName,
		"action":       action,
		"issue_number": issueNumber,
		"issue_title":  issueTitle,
		"issue_state":  issueState,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_issue_events_processed", 1, map[string]string{
		"repo":   event.RepoName,
		"action": action,
	})

	// Implement issue-specific logic based on the action
	switch action {
	case "opened":
		// Handle new issue
	case "closed":
		// Handle closed issue
	case "reopened":
		// Handle reopened issue
	case "assigned":
		// Handle issue assignment
	case "labeled", "unlabeled":
		// Handle label changes
	}

	return nil
}

// IssueCommentProcessor handles 'issue_comment' webhook events
type IssueCommentProcessor struct {
	BaseProcessor
}

// Process implements processing for issue comment events
func (p *IssueCommentProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid issue comment event: missing or invalid action")
	}

	// Extract the issue details
	issue, ok := payload["issue"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid issue comment event: missing or invalid issue object")
	}

	// Extract the comment details
	comment, ok := payload["comment"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid issue comment event: missing or invalid comment object")
	}

	issueNumber, ok := issue["number"].(float64)
	if !ok {
		return fmt.Errorf("invalid issue comment event: missing or invalid issue number")
	}

	commentID, ok := comment["id"].(float64)
	if !ok {
		return fmt.Errorf("invalid issue comment event: missing or invalid comment ID")
	}
	
	isPullRequest := issue["pull_request"] != nil
	
	p.Logger.Info("Processing issue comment event", map[string]interface{}{
		"delivery_id":    event.DeliveryID,
		"repo":           event.RepoName,
		"action":         action,
		"issue_number":   issueNumber,
		"comment_id":     commentID,
		"is_pull_request": isPullRequest,
		"user":           getMapStringValue(getNestedMap(comment, "user"), "login"),
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_issue_comment_events_processed", 1, map[string]string{
		"repo":           event.RepoName,
		"action":         action,
		"is_pull_request": fmt.Sprintf("%t", isPullRequest),
	})

	// Implement comment-specific logic based on the action
	switch action {
	case "created":
		// Handle new comment
		body := getMapStringValue(comment, "body")
		if strings.Contains(strings.ToLower(body), "/deploy") {
			// Example: Handle deployment command
			p.Logger.Info("Detected deployment command in comment", map[string]interface{}{
				"issue_number": issueNumber,
				"comment_id":   commentID,
				"user":         getMapStringValue(getNestedMap(comment, "user"), "login"),
			})
		}
	case "edited":
		// Handle edited comment
	case "deleted":
		// Handle deleted comment
	}

	return nil
}

// RepositoryProcessor handles 'repository' webhook events
type RepositoryProcessor struct {
	BaseProcessor
}

// Process implements processing for repository events
func (p *RepositoryProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid repository event: missing or invalid action")
	}

	// Extract the repository details
	repository, ok := payload["repository"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid repository event: missing or invalid repository object")
	}

	repoName := getMapStringValue(repository, "full_name")
	repoID, _ := repository["id"].(float64)
	
	p.Logger.Info("Processing repository event", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"repo":        repoName,
		"repo_id":     repoID,
		"action":      action,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_repository_events_processed", 1, map[string]string{
		"repo":   repoName,
		"action": action,
	})

	// Implement repository-specific logic based on the action
	switch action {
	case "created":
		// Handle new repository
	case "deleted":
		// Handle deleted repository
	case "archived":
		// Handle archived repository
	case "publicized":
		// Handle repository made public
	case "privatized":
		// Handle repository made private
	}

	return nil
}

// ReleaseProcessor handles 'release' webhook events
type ReleaseProcessor struct {
	BaseProcessor
}

// Process implements processing for release events
func (p *ReleaseProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid release event: missing or invalid action")
	}

	// Extract the release details
	release, ok := payload["release"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid release event: missing or invalid release object")
	}

	tagName := getMapStringValue(release, "tag_name")
	releaseName := getMapStringValue(release, "name")
	isDraft, _ := release["draft"].(bool)
	isPrerelease, _ := release["prerelease"].(bool)
	
	p.Logger.Info("Processing release event", map[string]interface{}{
		"delivery_id":  event.DeliveryID,
		"repo":         event.RepoName,
		"action":       action,
		"tag_name":     tagName,
		"release_name": releaseName,
		"is_draft":     isDraft,
		"is_prerelease": isPrerelease,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_release_events_processed", 1, map[string]string{
		"repo":   event.RepoName,
		"action": action,
	})

	// Implement release-specific logic based on the action
	switch action {
	case "published":
		// Handle published release
		// This is the most important action for releases - it means the release is public
		if !isDraft && !isPrerelease {
			// Handle formal release
			p.Logger.Info("Processing formal release", map[string]interface{}{
				"repo":      event.RepoName,
				"tag_name":  tagName,
				"published_at": getMapStringValue(release, "published_at"),
			})
		}
	case "created":
		// Handle created release (typically a draft)
	case "edited":
		// Handle edited release
	case "deleted":
		// Handle deleted release
	case "prereleased":
		// Handle pre-release
	}

	return nil
}

// WorkflowRunProcessor handles 'workflow_run' webhook events
type WorkflowRunProcessor struct {
	BaseProcessor
}

// Process implements processing for workflow run events
func (p *WorkflowRunProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid workflow_run event: missing or invalid action")
	}

	// Extract the workflow run details
	workflowRun, ok := payload["workflow_run"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid workflow_run event: missing or invalid workflow_run object")
	}

	workflowName := getMapStringValue(workflowRun, "name")
	workflowID, _ := workflowRun["id"].(float64)
	status := getMapStringValue(workflowRun, "status")
	conclusion := getMapStringValue(workflowRun, "conclusion")
	
	p.Logger.Info("Processing workflow run event", map[string]interface{}{
		"delivery_id":   event.DeliveryID,
		"repo":          event.RepoName,
		"action":        action,
		"workflow_name": workflowName,
		"workflow_id":   workflowID,
		"status":        status,
		"conclusion":    conclusion,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_workflow_run_events_processed", 1, map[string]string{
		"repo":    event.RepoName,
		"action":  action,
		"status":  status,
		"conclusion": conclusion,
	})

	return nil
}

// WorkflowJobProcessor handles 'workflow_job' webhook events
type WorkflowJobProcessor struct {
	BaseProcessor
}

// Process implements processing for workflow job events
func (p *WorkflowJobProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid workflow_job event: missing or invalid action")
	}

	// Extract the workflow job details
	workflowJob, ok := payload["workflow_job"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid workflow_job event: missing or invalid workflow_job object")
	}

	jobName := getMapStringValue(workflowJob, "name")
	jobID, _ := workflowJob["id"].(float64)
	status := getMapStringValue(workflowJob, "status")
	conclusion := getMapStringValue(workflowJob, "conclusion")
	
	p.Logger.Info("Processing workflow job event", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"repo":        event.RepoName,
		"action":      action,
		"job_name":    jobName,
		"job_id":      jobID,
		"status":      status,
		"conclusion":  conclusion,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_workflow_job_events_processed", 1, map[string]string{
		"repo":    event.RepoName,
		"action":  action,
		"status":  status,
		"conclusion": conclusion,
	})

	return nil
}

// CheckRunProcessor handles 'check_run' webhook events
type CheckRunProcessor struct {
	BaseProcessor
}

// Process implements processing for check run events
func (p *CheckRunProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid check_run event: missing or invalid action")
	}

	// Extract the check run details
	checkRun, ok := payload["check_run"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid check_run event: missing or invalid check_run object")
	}

	name := getMapStringValue(checkRun, "name")
	status := getMapStringValue(checkRun, "status")
	conclusion := getMapStringValue(checkRun, "conclusion")
	
	p.Logger.Info("Processing check run event", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"repo":        event.RepoName,
		"action":      action,
		"name":        name,
		"status":      status,
		"conclusion":  conclusion,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_check_run_events_processed", 1, map[string]string{
		"repo":   event.RepoName,
		"action": action,
		"status": status,
	})

	return nil
}

// DeploymentProcessor handles 'deployment' webhook events
type DeploymentProcessor struct {
	BaseProcessor
}

// Process implements processing for deployment events
func (p *DeploymentProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the deployment details
	deployment, ok := payload["deployment"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid deployment event: missing or invalid deployment object")
	}

	deploymentID, _ := deployment["id"].(float64)
	environment := getMapStringValue(deployment, "environment")
	ref := getMapStringValue(deployment, "ref")
	sha := getMapStringValue(deployment, "sha")
	
	p.Logger.Info("Processing deployment event", map[string]interface{}{
		"delivery_id":   event.DeliveryID,
		"repo":          event.RepoName,
		"deployment_id": deploymentID,
		"environment":   environment,
		"ref":           ref,
		"sha":           sha,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_deployment_events_processed", 1, map[string]string{
		"repo":        event.RepoName,
		"environment": environment,
	})

	return nil
}

// DeploymentStatusProcessor handles 'deployment_status' webhook events
type DeploymentStatusProcessor struct {
	BaseProcessor
}

// Process implements processing for deployment status events
func (p *DeploymentStatusProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the deployment details
	deployment, ok := payload["deployment"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid deployment_status event: missing or invalid deployment object")
	}

	// Extract the deployment status details
	deploymentStatus, ok := payload["deployment_status"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid deployment_status event: missing or invalid deployment_status object")
	}

	deploymentID, _ := deployment["id"].(float64)
	environment := getMapStringValue(deployment, "environment")
	state := getMapStringValue(deploymentStatus, "state")
	
	p.Logger.Info("Processing deployment status event", map[string]interface{}{
		"delivery_id":   event.DeliveryID,
		"repo":          event.RepoName,
		"deployment_id": deploymentID,
		"environment":   environment,
		"state":         state,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_deployment_status_events_processed", 1, map[string]string{
		"repo":        event.RepoName,
		"environment": environment,
		"state":       state,
	})

	return nil
}

// DependabotAlertProcessor handles 'dependabot_alert' webhook events
type DependabotAlertProcessor struct {
	BaseProcessor
}

// Process implements processing for dependabot alert events
func (p *DependabotAlertProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	// Extract the action from the payload
	action, ok := payload["action"].(string)
	if !ok {
		return fmt.Errorf("invalid dependabot_alert event: missing or invalid action")
	}

	// Extract the alert details
	alert, ok := payload["alert"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid dependabot_alert event: missing or invalid alert object")
	}

	alertNumber, _ := alert["number"].(float64)
	severity := getMapStringValue(alert, "security_advisory", "severity")
	packageName := getMapStringValue(alert, "dependency", "package", "name")
	
	p.Logger.Info("Processing dependabot alert event", map[string]interface{}{
		"delivery_id":  event.DeliveryID,
		"repo":         event.RepoName,
		"action":       action,
		"alert_number": alertNumber,
		"severity":     severity,
		"package":      packageName,
	})

	// Record metrics
	p.Metrics.IncrementCounter("github_dependabot_alert_events_processed", 1, map[string]string{
		"repo":     event.RepoName,
		"action":   action,
		"severity": severity,
	})

	return nil
}

// DefaultProcessor handles any webhook events that don't have a specific processor
type DefaultProcessor struct {
	BaseProcessor
}

// Process implements default processing for any event type
func (p *DefaultProcessor) Process(ctx context.Context, event queue.SQSEvent, payload map[string]interface{}) error {
	p.Logger.Info("Processing event with default processor", map[string]interface{}{
		"delivery_id": event.DeliveryID,
		"repo":        event.RepoName,
		"event_type":  event.EventType,
	})

	// Log the event type for monitoring (helps identify which events might need dedicated processors)
	p.Metrics.IncrementCounter("github_events_default_processor", 1, map[string]string{
		"event_type": event.EventType,
	})

	return nil
}

// Helper functions for extracting values from nested maps

// getMapStringValue safely extracts a string value from a map
func getMapStringValue(m map[string]interface{}, key string, nestedKeys ...string) string {
	// If we have a nested path, traverse it
	if len(nestedKeys) > 0 {
		current := m
		for _, k := range append([]string{key}, nestedKeys[:len(nestedKeys)-1]...) {
			if nextMap, ok := current[k].(map[string]interface{}); ok {
				current = nextMap
			} else {
				return ""
			}
		}
		lastKey := nestedKeys[len(nestedKeys)-1]
		if val, ok := current[lastKey].(string); ok {
			return val
		}
		return ""
	}

	// Simple key lookup
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// getNestedMap safely extracts a nested map
func getNestedMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return map[string]interface{}{}
}
