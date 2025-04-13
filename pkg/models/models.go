package models

import "time"

// GitHubQuery defines parameters for querying GitHub
type GitHubQuery struct {
	Type  string `json:"type"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	State string `json:"state"`
}

// GitHub query types
const (
	GitHubQueryTypeRepository   = "repository"
	GitHubQueryTypePullRequests = "pull_requests"
)

// HarnessQuery defines parameters for querying Harness
type HarnessQuery struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Harness query types
const (
	HarnessQueryTypePipeline      = "pipeline"
	HarnessQueryTypeCIBuild       = "ci_build"
	HarnessQueryTypeCDDeployment  = "cd_deployment"
	HarnessQueryTypeSTOExperiment = "sto_experiment"
	HarnessQueryTypeFeatureFlag   = "feature_flag"
)

// SonarQubeQuery defines parameters for querying SonarQube
type SonarQubeQuery struct {
	Type         string `json:"type"`
	ProjectKey   string `json:"projectKey"`
	Organization string `json:"organization"`
	Severity     string `json:"severity"`
	Status       string `json:"status"`
	MetricKeys   string `json:"metricKeys"`
}

// SonarQube query types
const (
	SonarQubeQueryTypeProject     = "project"
	SonarQubeQueryTypeQualityGate = "quality_gate"
	SonarQubeQueryTypeIssues      = "issues"
	SonarQubeQueryTypeMetrics     = "metrics"
)

// ArtifactoryQuery defines parameters for querying Artifactory
type ArtifactoryQuery struct {
	Type        string `json:"type"`
	RepoKey     string `json:"repoKey"`
	Path        string `json:"path"`
	RepoType    string `json:"repoType"`
	PackageType string `json:"packageType"`
	BuildName   string `json:"buildName"`
	BuildNumber string `json:"buildNumber"`
}

// Artifactory query types
const (
	ArtifactoryQueryTypeRepository = "repository"
	ArtifactoryQueryTypeArtifact   = "artifact"
	ArtifactoryQueryTypeBuild      = "build"
	ArtifactoryQueryTypeStorage    = "storage"
)

// XrayQuery defines parameters for querying JFrog Xray
type XrayQuery struct {
	Type         string `json:"type"`
	ArtifactPath string `json:"artifactPath"`
	CVE          string `json:"cve"`
	LicenseID    string `json:"licenseId"`
	BuildName    string `json:"buildName"`
	BuildNumber  string `json:"buildNumber"`
}

// Xray query types
const (
	XrayQueryTypeSummary         = "summary"
	XrayQueryTypeVulnerabilities = "vulnerabilities"
	XrayQueryTypeLicenses        = "licenses"
	XrayQueryTypeScans           = "scans"
)

// Basic event model types

// HarnessCIBuild represents a CI build in Harness
type HarnessCIBuild struct {
	ID          string    `json:"id"`
	BuildNumber int       `json:"buildNumber"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
}

// HarnessCIBuildEvent represents a CI build event from Harness
type HarnessCIBuildEvent struct {
	EventType string         `json:"eventType"`
	Build     HarnessCIBuild `json:"build"`
}

// HarnessCDDeployment represents a CD deployment in Harness
type HarnessCDDeployment struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Environment string    `json:"environment"`
	Service     string    `json:"service"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
}

// HarnessCDDeploymentEvent represents a CD deployment event from Harness
type HarnessCDDeploymentEvent struct {
	EventType  string              `json:"eventType"`
	Deployment HarnessCDDeployment `json:"deployment"`
}

// HarnessSTOExperiment represents an STO experiment in Harness
type HarnessSTOExperiment struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
}

// HarnessSTOExperimentEvent represents an STO experiment event from Harness
type HarnessSTOExperimentEvent struct {
	EventType  string               `json:"eventType"`
	Experiment HarnessSTOExperiment `json:"experiment"`
}

// HarnessFeatureFlag represents a feature flag in Harness
type HarnessFeatureFlag struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Status     string                 `json:"status"`
	Variations []interface{}          `json:"variations"`
	Rules      map[string]interface{} `json:"rules"`
}

// HarnessFeatureFlagEvent represents a feature flag event from Harness
type HarnessFeatureFlagEvent struct {
	EventType   string             `json:"eventType"`
	FeatureFlag HarnessFeatureFlag `json:"featureFlag"`
}

// HarnessPipeline represents a pipeline in Harness
type HarnessPipeline struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
}

// SonarQubeWebhookEvent represents a webhook event from SonarQube
type SonarQubeWebhookEvent struct {
	ServerURL   string                   `json:"serverUrl"`
	TaskID      string                   `json:"taskId"`
	Status      string                   `json:"status"`
	AnalysedAt  string                   `json:"analysedAt"`
	Project     *SonarQubeProjectRef     `json:"project,omitempty"`
	QualityGate *SonarQubeQualityGateRef `json:"qualityGate,omitempty"`
	Task        *SonarQubeTaskRef        `json:"task,omitempty"`
}

// SonarQubeProjectRef represents a project reference in a SonarQube webhook
type SonarQubeProjectRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// SonarQubeQualityGateRef represents a quality gate reference in a SonarQube webhook
type SonarQubeQualityGateRef struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// SonarQubeTaskRef represents a task reference in a SonarQube webhook
type SonarQubeTaskRef struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// ArtifactoryWebhookEvent represents a webhook event from Artifactory
type ArtifactoryWebhookEvent struct {
	Domain    string                 `json:"domain"`
	EventType string                 `json:"event_type"`
	Data      map[string]interface{} `json:"data"`
}

// XrayWebhookEvent represents a webhook event from JFrog Xray
type XrayWebhookEvent struct {
	EventType string                 `json:"event_type"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}
