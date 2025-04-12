package models

import "time"

// ===============================
// Harness Models
// ===============================

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

// HarnessPipeline represents a Harness pipeline
type HarnessPipeline struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Identifier     string    `json:"identifier"`
	ProjectID      string    `json:"projectId"`
	OrgID          string    `json:"orgId"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	LastRunAt      time.Time `json:"lastRunAt"`
	LastModifiedAt time.Time `json:"lastModifiedAt"`
	Tags           []string  `json:"tags"`
}

// HarnessCIBuild represents a Harness CI build
type HarnessCIBuild struct {
	ID          string    `json:"id"`
	BuildNumber int       `json:"buildNumber"`
	PipelineID  string    `json:"pipelineId"`
	CommitID    string    `json:"commitId"`
	Branch      string    `json:"branch"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	Duration    int64     `json:"duration"`
	TriggeredBy string    `json:"triggeredBy"`
}

// HarnessCDDeployment represents a Harness CD deployment
type HarnessCDDeployment struct {
	ID            string    `json:"id"`
	PipelineID    string    `json:"pipelineId"`
	ServiceID     string    `json:"serviceId"`
	EnvironmentID string    `json:"environmentId"`
	Status        string    `json:"status"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	ArtifactID    string    `json:"artifactId"`
	TriggeredBy   string    `json:"triggeredBy"`
}

// HarnessSTOExperiment represents a Harness STO experiment
type HarnessSTOExperiment struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	ServiceID     string    `json:"serviceId"`
	EnvironmentID string    `json:"environmentId"`
	Metrics       []string  `json:"metrics"`
}

// HarnessFeatureFlag represents a Harness feature flag
type HarnessFeatureFlag struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Identifier     string                 `json:"identifier"`
	ProjectID      string                 `json:"projectId"`
	Description    string                 `json:"description"`
	Variations     []interface{}          `json:"variations"`
	DefaultServeOn string                 `json:"defaultServeOn"`
	State          string                 `json:"state"`
	Kind           string                 `json:"kind"`
	Tags           []string               `json:"tags"`
	Targeting      map[string]interface{} `json:"targeting"`
}

// HarnessCIBuildEvent represents a Harness CI build webhook event
type HarnessCIBuildEvent struct {
	EventType string         `json:"eventType"`
	Build     HarnessCIBuild `json:"build"`
}

// HarnessCDDeploymentEvent represents a Harness CD deployment webhook event
type HarnessCDDeploymentEvent struct {
	EventType  string              `json:"eventType"`
	Deployment HarnessCDDeployment `json:"deployment"`
}

// HarnessSTOExperimentEvent represents a Harness STO experiment webhook event
type HarnessSTOExperimentEvent struct {
	EventType  string               `json:"eventType"`
	Experiment HarnessSTOExperiment `json:"experiment"`
}

// HarnessFeatureFlagEvent represents a Harness feature flag webhook event
type HarnessFeatureFlagEvent struct {
	EventType   string             `json:"eventType"`
	FeatureFlag HarnessFeatureFlag `json:"featureFlag"`
}

// ===============================
// SonarQube Models
// ===============================

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

// SonarQubeProject represents a SonarQube project
type SonarQubeProject struct {
	Key          string    `json:"key"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Qualifier    string    `json:"qualifier"`
	LastAnalysis time.Time `json:"lastAnalysisDate"`
	Visibility   string    `json:"visibility"`
}

// SonarQubeQualityGate represents a SonarQube quality gate
type SonarQubeQualityGate struct {
	ProjectStatus struct {
		Status      string               `json:"status"`
		Conditions  []SonarQubeCondition `json:"conditions"`
		PeriodIndex int                  `json:"periodIndex"`
	} `json:"projectStatus"`
}

// SonarQubeCondition represents a condition in a SonarQube quality gate
type SonarQubeCondition struct {
	Status         string `json:"status"`
	MetricKey      string `json:"metricKey"`
	Comparator     string `json:"comparator"`
	ErrorThreshold string `json:"errorThreshold"`
	ActualValue    string `json:"actualValue"`
}

// SonarQubeIssues represents issues in SonarQube
type SonarQubeIssues struct {
	Total  int              `json:"total"`
	Issues []SonarQubeIssue `json:"issues"`
}

// SonarQubeIssue represents a single issue in SonarQube
type SonarQubeIssue struct {
	Key          string    `json:"key"`
	Rule         string    `json:"rule"`
	Severity     string    `json:"severity"`
	Component    string    `json:"component"`
	Project      string    `json:"project"`
	Line         int       `json:"line"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
	Effort       string    `json:"effort"`
	Debt         string    `json:"debt"`
	CreationDate time.Time `json:"creationDate"`
}

// SonarQubeMetrics represents metrics in SonarQube
type SonarQubeMetrics struct {
	Component struct {
		Key       string             `json:"key"`
		Name      string             `json:"name"`
		Qualifier string             `json:"qualifier"`
		Measures  []SonarQubeMeasure `json:"measures"`
	} `json:"component"`
}

// SonarQubeMeasure represents a measure in SonarQube
type SonarQubeMeasure struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
	Period struct {
		Index int    `json:"index"`
		Value string `json:"value"`
	} `json:"period,omitempty"`
}

// SonarQubeWebhookEvent represents a SonarQube webhook event
type SonarQubeWebhookEvent struct {
	ServerURL   string                   `json:"serverUrl"`
	TaskID      string                   `json:"taskId"`
	Status      string                   `json:"status"`
	AnalysedAt  string                   `json:"analysedAt"`
	Project     *SonarQubeProjectRef     `json:"project,omitempty"`
	QualityGate *SonarQubeQualityGateRef `json:"qualityGate,omitempty"`
	Branch      *SonarQubeBranchRef      `json:"branch,omitempty"`
	Task        *SonarQubeTaskRef        `json:"task,omitempty"`
}

// SonarQubeProjectRef represents a project reference in a webhook
type SonarQubeProjectRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// SonarQubeQualityGateRef represents a quality gate reference in a webhook
type SonarQubeQualityGateRef struct {
	Name       string                  `json:"name"`
	Status     string                  `json:"status"`
	Conditions []SonarQubeConditionRef `json:"conditions"`
}

// SonarQubeConditionRef represents a condition reference in a webhook
type SonarQubeConditionRef struct {
	Metric         string `json:"metric"`
	Status         string `json:"status"`
	ErrorThreshold string `json:"errorThreshold"`
	ActualValue    string `json:"actualValue"`
}

// SonarQubeBranchRef represents a branch reference in a webhook
type SonarQubeBranchRef struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// SonarQubeTaskRef represents a task reference in a webhook
type SonarQubeTaskRef struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	StartedAt       string `json:"startedAt"`
	ExecutionTimeMs int64  `json:"executionTimeMs"`
}

// ===============================
// Artifactory Models
// ===============================

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

// ArtifactoryRepositories represents repositories in Artifactory
type ArtifactoryRepositories struct {
	Repositories []ArtifactoryRepository `json:"repositories"`
}

// ArtifactoryRepository represents a single repository in Artifactory
type ArtifactoryRepository struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Description string `json:"description"`
	URL         string `json:"url"`
	PackageType string `json:"packageType"`
}

// ArtifactoryArtifact represents an artifact in Artifactory
type ArtifactoryArtifact struct {
	Repo        string               `json:"repo"`
	Path        string               `json:"path"`
	Name        string               `json:"name"`
	Type        string               `json:"type"`
	Size        int64                `json:"size"`
	Created     string               `json:"created"`
	CreatedBy   string               `json:"createdBy"`
	Modified    string               `json:"modified"`
	ModifiedBy  string               `json:"modifiedBy"`
	LastUpdated string               `json:"lastUpdated"`
	DownloadUri string               `json:"downloadUri"`
	MimeType    string               `json:"mimeType"`
	Checksums   ArtifactoryChecksums `json:"checksums"`
	Properties  map[string][]string  `json:"properties"`
}

// ArtifactoryChecksums represents checksums for an artifact
type ArtifactoryChecksums struct {
	SHA1   string `json:"sha1"`
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

// ArtifactoryBuild represents a build in Artifactory
type ArtifactoryBuild struct {
	BuildInfo struct {
		Name       string            `json:"name"`
		Number     string            `json:"number"`
		Started    time.Time         `json:"started"`
		BuildAgent BuildAgent        `json:"buildAgent"`
		Modules    []Module          `json:"modules"`
		Properties map[string]string `json:"properties"`
	} `json:"buildInfo"`
}

// BuildAgent represents a build agent in Artifactory
type BuildAgent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Module represents a module in an Artifactory build
type Module struct {
	ID           string       `json:"id"`
	Artifacts    []Artifact   `json:"artifacts"`
	Dependencies []Dependency `json:"dependencies"`
}

// Artifact represents an artifact in an Artifactory build module
type Artifact struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	SHA1   string `json:"sha1"`
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

// Dependency represents a dependency in an Artifactory build module
type Dependency struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	SHA1   string `json:"sha1"`
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

// ArtifactoryStorage represents storage info in Artifactory
type ArtifactoryStorage struct {
	BinariesSummary struct {
		BinariesCount int64 `json:"binariesCount"`
		BinariesSize  int64 `json:"binariesSize"`
		ArtifactsSize int64 `json:"artifactsSize"`
		Optimization  int64 `json:"optimization"`
		ItemsCount    int64 `json:"itemsCount"`
	} `json:"binariesSummary"`
	FileStoreSummary struct {
		StorageType      string `json:"storageType"`
		StorageDirectory string `json:"storageDirectory"`
		TotalSpace       int64  `json:"totalSpace"`
		UsedSpace        int64  `json:"usedSpace"`
		FreeSpace        int64  `json:"freeSpace"`
	} `json:"fileStoreSummary"`
	RepositoriesSummary struct {
		RepoCount    int           `json:"repoCount"`
		Repositories []RepoSummary `json:"repositories"`
	} `json:"repositoriesSummary"`
}

// RepoSummary represents a repository summary in Artifactory
type RepoSummary struct {
	RepoKey      string `json:"repoKey"`
	RepoType     string `json:"repoType"`
	FoldersCount int    `json:"foldersCount"`
	FilesCount   int    `json:"filesCount"`
	UsedSpace    string `json:"usedSpace"`
	ItemsCount   int    `json:"itemsCount"`
	PackageType  string `json:"packageType"`
	Percentage   string `json:"percentage"`
}

// ArtifactoryWebhookEvent represents an Artifactory webhook event
type ArtifactoryWebhookEvent struct {
	Domain    string                 `json:"domain"`
	EventType string                 `json:"event_type"`
	Data      ArtifactoryWebhookData `json:"data"`
}

// ArtifactoryWebhookData represents the data in an Artifactory webhook event
type ArtifactoryWebhookData struct {
	RepoKey    string `json:"repo_key"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	SHA1       string `json:"sha1"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	Created    string `json:"created"`
	CreatedBy  string `json:"created_by"`
	Modified   string `json:"modified"`
	ModifiedBy string `json:"modified_by"`
}

// ===============================
// JFrog Xray Models
// ===============================

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

// XraySummary represents a summary of threats in Xray
type XraySummary struct {
	Vulnerabilities    []XrayVulnerability `json:"vulnerabilities"`
	Licenses           []XrayLicense       `json:"licenses"`
	SecurityViolations []XrayViolation     `json:"security_violations"`
	LicenseViolations  []XrayViolation     `json:"license_violations"`
}

// XrayVulnerability represents a vulnerability in Xray
type XrayVulnerability struct {
	CVE           string   `json:"cve"`
	Summary       string   `json:"summary"`
	Severity      string   `json:"severity"`
	ImpactPath    []string `json:"impact_path"`
	FixedVersions []string `json:"fixed_versions"`
	Components    []string `json:"components"`
}

// XrayLicense represents a license in Xray
type XrayLicense struct {
	Name        string   `json:"name"`
	FullName    string   `json:"full_name"`
	MoreInfoURL string   `json:"more_info_url"`
	Components  []string `json:"components"`
}

// XrayViolation represents a violation in Xray
type XrayViolation struct {
	Type       string   `json:"type"`
	PolicyName string   `json:"policy_name"`
	WatchName  string   `json:"watch_name"`
	Components []string `json:"components"`
}

// XrayVulnerabilities represents vulnerabilities in Xray
type XrayVulnerabilities struct {
	Vulnerabilities []XrayVulnerabilityDetail `json:"vulnerabilities"`
	TotalCount      int                       `json:"total_count"`
}

// XrayVulnerabilityDetail represents a detailed vulnerability in Xray
type XrayVulnerabilityDetail struct {
	ID            string   `json:"id"`
	Summary       string   `json:"summary"`
	Description   string   `json:"description"`
	CvssV2Score   float64  `json:"cvss_v2_score"`
	CvssV3Score   float64  `json:"cvss_v3_score"`
	Severity      string   `json:"severity"`
	PublishedDate string   `json:"published_date"`
	LastUpdated   string   `json:"last_updated"`
	References    []string `json:"references"`
	Remediation   string   `json:"remediation"`
}

// XrayLicenses represents licenses in Xray
type XrayLicenses struct {
	Licenses   []XrayLicenseDetail `json:"licenses"`
	TotalCount int                 `json:"total_count"`
}

// XrayLicenseDetail represents a detailed license in Xray
type XrayLicenseDetail struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	FullName    string                 `json:"full_name"`
	URL         string                 `json:"url"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Components  []XrayLicenseComponent `json:"components"`
}

// XrayLicenseComponent represents a component with a license in Xray
type XrayLicenseComponent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// XrayScans represents scans in Xray
type XrayScans struct {
	Scans []XrayScan `json:"scans"`
}

// XrayScan represents a scan in Xray
type XrayScan struct {
	ID                 string    `json:"id"`
	ResourceType       string    `json:"resource_type"`
	ResourceName       string    `json:"resource_name"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	VulnerabilityCount int       `json:"vulnerability_count"`
	LicenseCount       int       `json:"license_count"`
}

// XrayWebhookEvent represents a JFrog Xray webhook event
type XrayWebhookEvent struct {
	EventType string          `json:"event_type"`
	Timestamp string          `json:"timestamp"`
	Data      XrayWebhookData `json:"data"`
}

// XrayWebhookData represents the data in a JFrog Xray webhook event
type XrayWebhookData struct {
	Issues     []XrayIssue `json:"issues"`
	ProjectKey string      `json:"project_key"`
	WatchName  string      `json:"watch_name"`
	PolicyName string      `json:"policy_name"`
}

// XrayIssue represents an issue in a JFrog Xray webhook event
type XrayIssue struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	Severity  string             `json:"severity"`
	Summary   string             `json:"summary"`
	Component XrayIssueComponent `json:"component"`
}

// XrayIssueComponent represents a component in a JFrog Xray issue
type XrayIssueComponent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}
