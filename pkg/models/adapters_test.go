package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestHarnessModels tests the Harness model structures
func TestHarnessModels(t *testing.T) {
	t.Run("HarnessQuery", func(t *testing.T) {
		query := HarnessQuery{
			Type: HarnessQueryTypePipeline,
			ID:   "pipeline-123",
		}
		
		// Test basic field access
		assert.Equal(t, HarnessQueryTypePipeline, query.Type)
		assert.Equal(t, "pipeline-123", query.ID)
		
		// Test query type constants
		assert.Equal(t, "pipeline", HarnessQueryTypePipeline)
		assert.Equal(t, "ci_build", HarnessQueryTypeCIBuild)
		assert.Equal(t, "cd_deployment", HarnessQueryTypeCDDeployment)
		assert.Equal(t, "sto_experiment", HarnessQueryTypeSTOExperiment)
		assert.Equal(t, "feature_flag", HarnessQueryTypeFeatureFlag)
	})
	
	t.Run("HarnessPipeline", func(t *testing.T) {
		now := time.Now()
		pipeline := HarnessPipeline{
			ID:             "pipeline-123",
			Name:           "Test Pipeline",
			Identifier:     "test_pipeline",
			ProjectID:      "project-123",
			OrgID:          "org-123",
			Status:         "active",
			CreatedAt:      now,
			LastRunAt:      now,
			LastModifiedAt: now,
			Tags:           []string{"test", "pipeline"},
		}
		
		// Test basic field access
		assert.Equal(t, "pipeline-123", pipeline.ID)
		assert.Equal(t, "Test Pipeline", pipeline.Name)
		assert.Equal(t, "test_pipeline", pipeline.Identifier)
		assert.Equal(t, "project-123", pipeline.ProjectID)
		assert.Equal(t, "org-123", pipeline.OrgID)
		assert.Equal(t, "active", pipeline.Status)
		assert.Equal(t, now, pipeline.CreatedAt)
		assert.Equal(t, now, pipeline.LastRunAt)
		assert.Equal(t, now, pipeline.LastModifiedAt)
		assert.Equal(t, []string{"test", "pipeline"}, pipeline.Tags)
	})
	
	t.Run("HarnessCIBuild", func(t *testing.T) {
		now := time.Now()
		build := HarnessCIBuild{
			ID:          "build-123",
			BuildNumber: 42,
			PipelineID:  "pipeline-123",
			CommitID:    "abc123",
			Branch:      "main",
			Status:      "success",
			StartTime:   now.Add(-10 * time.Minute),
			EndTime:     now,
			Duration:    600000, // 10 minutes in ms
			TriggeredBy: "user-123",
		}
		
		// Test basic field access
		assert.Equal(t, "build-123", build.ID)
		assert.Equal(t, 42, build.BuildNumber)
		assert.Equal(t, "pipeline-123", build.PipelineID)
		assert.Equal(t, "abc123", build.CommitID)
		assert.Equal(t, "main", build.Branch)
		assert.Equal(t, "success", build.Status)
		assert.Equal(t, now.Add(-10*time.Minute), build.StartTime)
		assert.Equal(t, now, build.EndTime)
		assert.Equal(t, int64(600000), build.Duration)
		assert.Equal(t, "user-123", build.TriggeredBy)
	})
	
	// Test serialization and deserialization
	t.Run("HarnessSerialization", func(t *testing.T) {
		now := time.Now()
		deployment := HarnessCDDeployment{
			ID:            "deploy-123",
			PipelineID:    "pipeline-123",
			ServiceID:     "service-123",
			EnvironmentID: "env-123",
			Environment:   "production",
			Service:       "api-service",
			Status:        "success",
			StartTime:     now.Add(-5 * time.Minute),
			EndTime:       now,
			ArtifactID:    "artifact-123",
			TriggeredBy:   "user-123",
		}
		
		// Serialize to JSON
		jsonData, err := json.Marshal(deployment)
		assert.NoError(t, err)
		
		// Deserialize from JSON
		var deserializedDeployment HarnessCDDeployment
		err = json.Unmarshal(jsonData, &deserializedDeployment)
		assert.NoError(t, err)
		
		// Verify fields match
		assert.Equal(t, deployment.ID, deserializedDeployment.ID)
		assert.Equal(t, deployment.PipelineID, deserializedDeployment.PipelineID)
		assert.Equal(t, deployment.ServiceID, deserializedDeployment.ServiceID)
		assert.Equal(t, deployment.EnvironmentID, deserializedDeployment.EnvironmentID)
		assert.Equal(t, deployment.Environment, deserializedDeployment.Environment)
		assert.Equal(t, deployment.Service, deserializedDeployment.Service)
		assert.Equal(t, deployment.Status, deserializedDeployment.Status)
		assert.Equal(t, deployment.ArtifactID, deserializedDeployment.ArtifactID)
		assert.Equal(t, deployment.TriggeredBy, deserializedDeployment.TriggeredBy)
	})
}

// TestSonarQubeModels tests the SonarQube model structures
func TestSonarQubeModels(t *testing.T) {
	t.Run("SonarQubeQuery", func(t *testing.T) {
		query := SonarQubeQuery{
			Type:         SonarQubeQueryTypeProject,
			ProjectKey:   "project-key",
			Organization: "org-key",
			Severity:     "CRITICAL",
			Status:       "OPEN",
			MetricKeys:   "complexity,coverage",
		}
		
		// Test basic field access
		assert.Equal(t, SonarQubeQueryTypeProject, query.Type)
		assert.Equal(t, "project-key", query.ProjectKey)
		assert.Equal(t, "org-key", query.Organization)
		assert.Equal(t, "CRITICAL", query.Severity)
		assert.Equal(t, "OPEN", query.Status)
		assert.Equal(t, "complexity,coverage", query.MetricKeys)
		
		// Test query type constants
		assert.Equal(t, "project", SonarQubeQueryTypeProject)
		assert.Equal(t, "quality_gate", SonarQubeQueryTypeQualityGate)
		assert.Equal(t, "issues", SonarQubeQueryTypeIssues)
		assert.Equal(t, "metrics", SonarQubeQueryTypeMetrics)
	})
	
	t.Run("SonarQubeProject", func(t *testing.T) {
		now := time.Now()
		project := SonarQubeProject{
			Key:          "project-key",
			Name:         "Test Project",
			Description:  "Test project description",
			Qualifier:    "TRK",
			LastAnalysis: now,
			Visibility:   "public",
		}
		
		// Test basic field access
		assert.Equal(t, "project-key", project.Key)
		assert.Equal(t, "Test Project", project.Name)
		assert.Equal(t, "Test project description", project.Description)
		assert.Equal(t, "TRK", project.Qualifier)
		assert.Equal(t, now, project.LastAnalysis)
		assert.Equal(t, "public", project.Visibility)
	})
	
	t.Run("SonarQubeWebhookEvent", func(t *testing.T) {
		// Create a test webhook event
		event := SonarQubeWebhookEvent{
			ServerURL:  "https://sonarqube.example.com",
			TaskID:     "task-123",
			Status:     "SUCCESS",
			AnalysedAt: "2023-01-01T00:00:00Z",
			Project: &SonarQubeProjectRef{
				Key:  "project-key",
				Name: "Test Project",
				URL:  "https://sonarqube.example.com/dashboard?id=project-key",
			},
			QualityGate: &SonarQubeQualityGateRef{
				Name:   "Default",
				Status: "OK",
				Conditions: []SonarQubeConditionRef{
					{
						Metric:         "coverage",
						Status:         "OK",
						ErrorThreshold: "80",
						ActualValue:    "85",
					},
				},
			},
			Branch: &SonarQubeBranchRef{
				Name: "main",
				Type: "LONG",
				URL:  "https://sonarqube.example.com/dashboard?id=project-key&branch=main",
			},
			Task: &SonarQubeTaskRef{
				ID:              "task-123",
				Type:            "REPORT",
				Status:          "SUCCESS",
				StartedAt:       "2023-01-01T00:00:00Z",
				ExecutionTimeMs: 5000,
			},
		}
		
		// Test basic field access
		assert.Equal(t, "https://sonarqube.example.com", event.ServerURL)
		assert.Equal(t, "task-123", event.TaskID)
		assert.Equal(t, "SUCCESS", event.Status)
		assert.Equal(t, "2023-01-01T00:00:00Z", event.AnalysedAt)
		
		// Test nested objects
		assert.NotNil(t, event.Project)
		assert.Equal(t, "project-key", event.Project.Key)
		
		assert.NotNil(t, event.QualityGate)
		assert.Equal(t, "Default", event.QualityGate.Name)
		assert.Equal(t, "OK", event.QualityGate.Status)
		assert.Equal(t, 1, len(event.QualityGate.Conditions))
		
		assert.NotNil(t, event.Branch)
		assert.Equal(t, "main", event.Branch.Name)
		
		assert.NotNil(t, event.Task)
		assert.Equal(t, "task-123", event.Task.ID)
		assert.Equal(t, int64(5000), event.Task.ExecutionTimeMs)
	})
	
	// Test serialization and deserialization
	t.Run("SonarQubeSerialization", func(t *testing.T) {
		// Create a test issue
		issue := SonarQubeIssue{
			Key:          "issue-123",
			Rule:         "rule-123",
			Severity:     "CRITICAL",
			Component:    "project-key:src/main.js",
			Project:      "project-key",
			Line:         42,
			Status:       "OPEN",
			Message:      "Fix this security issue",
			Effort:       "5min",
			Debt:         "5min",
			CreationDate: time.Now(),
		}
		
		// Serialize to JSON
		jsonData, err := json.Marshal(issue)
		assert.NoError(t, err)
		
		// Deserialize from JSON
		var deserializedIssue SonarQubeIssue
		err = json.Unmarshal(jsonData, &deserializedIssue)
		assert.NoError(t, err)
		
		// Verify fields match
		assert.Equal(t, issue.Key, deserializedIssue.Key)
		assert.Equal(t, issue.Rule, deserializedIssue.Rule)
		assert.Equal(t, issue.Severity, deserializedIssue.Severity)
		assert.Equal(t, issue.Component, deserializedIssue.Component)
		assert.Equal(t, issue.Project, deserializedIssue.Project)
		assert.Equal(t, issue.Line, deserializedIssue.Line)
		assert.Equal(t, issue.Status, deserializedIssue.Status)
		assert.Equal(t, issue.Message, deserializedIssue.Message)
		assert.Equal(t, issue.Effort, deserializedIssue.Effort)
	})
}

// TestArtifactoryModels tests the Artifactory model structures
func TestArtifactoryModels(t *testing.T) {
	t.Run("ArtifactoryQuery", func(t *testing.T) {
		query := ArtifactoryQuery{
			Type:        ArtifactoryQueryTypeRepository,
			RepoKey:     "repo-key",
			Path:        "path/to/artifact",
			RepoType:    "local",
			PackageType: "npm",
			BuildName:   "build-name",
			BuildNumber: "1.0.0",
		}
		
		// Test basic field access
		assert.Equal(t, ArtifactoryQueryTypeRepository, query.Type)
		assert.Equal(t, "repo-key", query.RepoKey)
		assert.Equal(t, "path/to/artifact", query.Path)
		assert.Equal(t, "local", query.RepoType)
		assert.Equal(t, "npm", query.PackageType)
		assert.Equal(t, "build-name", query.BuildName)
		assert.Equal(t, "1.0.0", query.BuildNumber)
		
		// Test query type constants
		assert.Equal(t, "repository", ArtifactoryQueryTypeRepository)
		assert.Equal(t, "artifact", ArtifactoryQueryTypeArtifact)
		assert.Equal(t, "build", ArtifactoryQueryTypeBuild)
		assert.Equal(t, "storage", ArtifactoryQueryTypeStorage)
	})
	
	t.Run("ArtifactoryRepository", func(t *testing.T) {
		repo := ArtifactoryRepository{
			Key:         "repo-key",
			Type:        "local",
			Description: "Test repository",
			URL:         "https://artifactory.example.com/artifactory/repo-key",
			PackageType: "npm",
		}
		
		// Test basic field access
		assert.Equal(t, "repo-key", repo.Key)
		assert.Equal(t, "local", repo.Type)
		assert.Equal(t, "Test repository", repo.Description)
		assert.Equal(t, "https://artifactory.example.com/artifactory/repo-key", repo.URL)
		assert.Equal(t, "npm", repo.PackageType)
	})
	
	t.Run("ArtifactoryArtifact", func(t *testing.T) {
		// Create a test artifact
		artifact := ArtifactoryArtifact{
			Repo:        "repo-key",
			Path:        "path/to",
			Name:        "artifact.zip",
			Type:        "file",
			Size:        1024,
			Created:     "2023-01-01T00:00:00Z",
			CreatedBy:   "user-123",
			Modified:    "2023-01-02T00:00:00Z",
			ModifiedBy:  "user-456",
			LastUpdated: "2023-01-02T00:00:00Z",
			DownloadUri: "https://artifactory.example.com/artifactory/repo-key/path/to/artifact.zip",
			MimeType:    "application/zip",
			Checksums: ArtifactoryChecksums{
				SHA1:   "sha1-hash",
				MD5:    "md5-hash",
				SHA256: "sha256-hash",
			},
			Properties: map[string][]string{
				"key1": {"value1"},
				"key2": {"value2", "value3"},
			},
		}
		
		// Test basic field access
		assert.Equal(t, "repo-key", artifact.Repo)
		assert.Equal(t, "path/to", artifact.Path)
		assert.Equal(t, "artifact.zip", artifact.Name)
		assert.Equal(t, "file", artifact.Type)
		assert.Equal(t, int64(1024), artifact.Size)
		assert.Equal(t, "2023-01-01T00:00:00Z", artifact.Created)
		assert.Equal(t, "user-123", artifact.CreatedBy)
		assert.Equal(t, "2023-01-02T00:00:00Z", artifact.Modified)
		assert.Equal(t, "user-456", artifact.ModifiedBy)
		assert.Equal(t, "2023-01-02T00:00:00Z", artifact.LastUpdated)
		assert.Equal(t, "https://artifactory.example.com/artifactory/repo-key/path/to/artifact.zip", artifact.DownloadUri)
		assert.Equal(t, "application/zip", artifact.MimeType)
		
		// Test nested objects
		assert.Equal(t, "sha1-hash", artifact.Checksums.SHA1)
		assert.Equal(t, "md5-hash", artifact.Checksums.MD5)
		assert.Equal(t, "sha256-hash", artifact.Checksums.SHA256)
		
		// Test properties
		assert.Equal(t, []string{"value1"}, artifact.Properties["key1"])
		assert.Equal(t, []string{"value2", "value3"}, artifact.Properties["key2"])
	})
	
	// Test webhook event
	t.Run("ArtifactoryWebhookEvent", func(t *testing.T) {
		// Create a test webhook event
		event := ArtifactoryWebhookEvent{
			Domain:    "artifact",
			EventType: "deployed",
			Data: ArtifactoryWebhookData{
				RepoKey:    "repo-key",
				Path:       "path/to",
				Name:       "artifact.zip",
				SHA1:       "sha1-hash",
				SHA256:     "sha256-hash",
				Size:       1024,
				Created:    "2023-01-01T00:00:00Z",
				CreatedBy:  "user-123",
				Modified:   "2023-01-02T00:00:00Z",
				ModifiedBy: "user-456",
			},
		}
		
		// Test basic field access
		assert.Equal(t, "artifact", event.Domain)
		assert.Equal(t, "deployed", event.EventType)
		
		// Test data field
		assert.Equal(t, "repo-key", event.Data.RepoKey)
		assert.Equal(t, "path/to", event.Data.Path)
		assert.Equal(t, "artifact.zip", event.Data.Name)
		assert.Equal(t, "sha1-hash", event.Data.SHA1)
		assert.Equal(t, "sha256-hash", event.Data.SHA256)
		assert.Equal(t, int64(1024), event.Data.Size)
		assert.Equal(t, "2023-01-01T00:00:00Z", event.Data.Created)
		assert.Equal(t, "user-123", event.Data.CreatedBy)
		assert.Equal(t, "2023-01-02T00:00:00Z", event.Data.Modified)
		assert.Equal(t, "user-456", event.Data.ModifiedBy)
	})
}

// TestXrayModels tests the JFrog Xray model structures
func TestXrayModels(t *testing.T) {
	t.Run("XrayQuery", func(t *testing.T) {
		query := XrayQuery{
			Type:         XrayQueryTypeSummary,
			ArtifactPath: "repo-key/path/to/artifact.zip",
			CVE:          "CVE-2023-12345",
			LicenseID:    "MIT",
			BuildName:    "build-name",
			BuildNumber:  "1.0.0",
		}
		
		// Test basic field access
		assert.Equal(t, XrayQueryTypeSummary, query.Type)
		assert.Equal(t, "repo-key/path/to/artifact.zip", query.ArtifactPath)
		assert.Equal(t, "CVE-2023-12345", query.CVE)
		assert.Equal(t, "MIT", query.LicenseID)
		assert.Equal(t, "build-name", query.BuildName)
		assert.Equal(t, "1.0.0", query.BuildNumber)
		
		// Test query type constants
		assert.Equal(t, "summary", XrayQueryTypeSummary)
		assert.Equal(t, "vulnerabilities", XrayQueryTypeVulnerabilities)
		assert.Equal(t, "licenses", XrayQueryTypeLicenses)
		assert.Equal(t, "scans", XrayQueryTypeScans)
	})
	
	t.Run("XraySummary", func(t *testing.T) {
		// Create a test summary
		summary := XraySummary{
			Vulnerabilities: []XrayVulnerability{
				{
					CVE:           "CVE-2023-12345",
					Summary:       "Test vulnerability",
					Severity:      "high",
					ImpactPath:    []string{"path1", "path2"},
					FixedVersions: []string{"1.2.0", "1.3.0"},
					Components:    []string{"component1", "component2"},
				},
			},
			Licenses: []XrayLicense{
				{
					Name:        "MIT",
					FullName:    "MIT License",
					MoreInfoURL: "https://opensource.org/licenses/MIT",
					Components:  []string{"component1"},
				},
			},
			SecurityViolations: []XrayViolation{
				{
					Type:       "security",
					PolicyName: "security-policy",
					WatchName:  "security-watch",
					Components: []string{"component1"},
				},
			},
			LicenseViolations: []XrayViolation{
				{
					Type:       "license",
					PolicyName: "license-policy",
					WatchName:  "license-watch",
					Components: []string{"component2"},
				},
			},
		}
		
		// Test basic field access
		assert.Equal(t, 1, len(summary.Vulnerabilities))
		assert.Equal(t, "CVE-2023-12345", summary.Vulnerabilities[0].CVE)
		assert.Equal(t, "high", summary.Vulnerabilities[0].Severity)
		
		assert.Equal(t, 1, len(summary.Licenses))
		assert.Equal(t, "MIT", summary.Licenses[0].Name)
		assert.Equal(t, "MIT License", summary.Licenses[0].FullName)
		
		assert.Equal(t, 1, len(summary.SecurityViolations))
		assert.Equal(t, "security", summary.SecurityViolations[0].Type)
		assert.Equal(t, "security-policy", summary.SecurityViolations[0].PolicyName)
		
		assert.Equal(t, 1, len(summary.LicenseViolations))
		assert.Equal(t, "license", summary.LicenseViolations[0].Type)
		assert.Equal(t, "license-policy", summary.LicenseViolations[0].PolicyName)
	})
	
	// Test webhook event
	t.Run("XrayWebhookEvent", func(t *testing.T) {
		// Create a test webhook event
		event := XrayWebhookEvent{
			EventType: "policy_violation",
			Timestamp: "2023-01-01T00:00:00Z",
			Data: XrayWebhookData{
				Issues: []XrayIssue{
					{
						ID:       "issue-123",
						Type:     "security",
						Severity: "high",
						Summary:  "Security vulnerability detected",
						Component: XrayIssueComponent{
							Name:    "component-name",
							Version: "1.0.0",
							Path:    "repo-key/path/to/component",
						},
					},
				},
				ProjectKey: "project-key",
				WatchName:  "security-watch",
				PolicyName: "security-policy",
			},
		}
		
		// Test basic field access
		assert.Equal(t, "policy_violation", event.EventType)
		assert.Equal(t, "2023-01-01T00:00:00Z", event.Timestamp)
		
		// Test data field
		assert.Equal(t, 1, len(event.Data.Issues))
		assert.Equal(t, "issue-123", event.Data.Issues[0].ID)
		assert.Equal(t, "security", event.Data.Issues[0].Type)
		assert.Equal(t, "high", event.Data.Issues[0].Severity)
		assert.Equal(t, "Security vulnerability detected", event.Data.Issues[0].Summary)
		
		assert.Equal(t, "component-name", event.Data.Issues[0].Component.Name)
		assert.Equal(t, "1.0.0", event.Data.Issues[0].Component.Version)
		assert.Equal(t, "repo-key/path/to/component", event.Data.Issues[0].Component.Path)
		
		assert.Equal(t, "project-key", event.Data.ProjectKey)
		assert.Equal(t, "security-watch", event.Data.WatchName)
		assert.Equal(t, "security-policy", event.Data.PolicyName)
	})
}
