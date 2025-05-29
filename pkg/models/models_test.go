package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessCIBuild(t *testing.T) {
	// Test HarnessCIBuild serialization
	now := time.Now().UTC()
	build := HarnessCIBuild{
		ID:          "build-123",
		BuildNumber: 42,
		Status:      "success",
		StartTime:   now.Add(-30 * time.Minute),
		EndTime:     now,
	}

	// Test marshaling
	data, err := json.Marshal(build)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded HarnessCIBuild
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, build.ID, decoded.ID)
	assert.Equal(t, build.BuildNumber, decoded.BuildNumber)
	assert.Equal(t, build.Status, decoded.Status)
	assert.Equal(t, build.StartTime.Unix(), decoded.StartTime.Unix())
	assert.Equal(t, build.EndTime.Unix(), decoded.EndTime.Unix())
}

func TestHarnessCIBuildEvent(t *testing.T) {
	// Test HarnessCIBuildEvent serialization
	now := time.Now().UTC()
	event := HarnessCIBuildEvent{
		EventType: "ci.build",
		Build: HarnessCIBuild{
			ID:          "build-123",
			BuildNumber: 42,
			Status:      "success",
			StartTime:   now.Add(-30 * time.Minute),
			EndTime:     now,
		},
	}

	// Test marshaling
	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded HarnessCIBuildEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.EventType, decoded.EventType)
	assert.Equal(t, event.Build.ID, decoded.Build.ID)
	assert.Equal(t, event.Build.BuildNumber, decoded.Build.BuildNumber)
	assert.Equal(t, event.Build.Status, decoded.Build.Status)
}

func TestHarnessCDDeployment(t *testing.T) {
	// Test HarnessCDDeployment serialization
	now := time.Now().UTC()
	deployment := HarnessCDDeployment{
		ID:          "deploy-123",
		Status:      "success",
		Environment: "production",
		Service:     "api-service",
		StartTime:   now.Add(-30 * time.Minute),
		EndTime:     now,
	}

	// Test marshaling
	data, err := json.Marshal(deployment)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded HarnessCDDeployment
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, deployment.ID, decoded.ID)
	assert.Equal(t, deployment.Status, decoded.Status)
	assert.Equal(t, deployment.Environment, decoded.Environment)
	assert.Equal(t, deployment.Service, decoded.Service)
	assert.Equal(t, deployment.StartTime.Unix(), decoded.StartTime.Unix())
	assert.Equal(t, deployment.EndTime.Unix(), decoded.EndTime.Unix())
}

func TestSonarQubeWebhookEvent(t *testing.T) {
	// Test SonarQubeWebhookEvent serialization
	event := SonarQubeWebhookEvent{
		ServerURL:  "https://sonarqube.example.com",
		TaskID:     "task-123",
		Status:     "SUCCESS",
		AnalysedAt: "2023-01-02T15:30:45Z",
		Project: &SonarQubeProjectRef{
			Key:  "org:project",
			Name: "Test Project",
			URL:  "https://sonarqube.example.com/dashboard?id=org%3Aproject",
		},
		QualityGate: &SonarQubeQualityGateRef{
			Name:   "Default",
			Status: "OK",
		},
		Task: &SonarQubeTaskRef{
			ID:     "task-123",
			Type:   "REPORT",
			Status: "SUCCESS",
		},
	}

	// Test marshaling
	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded SonarQubeWebhookEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.ServerURL, decoded.ServerURL)
	assert.Equal(t, event.TaskID, decoded.TaskID)
	assert.Equal(t, event.Status, decoded.Status)
	assert.Equal(t, event.AnalysedAt, decoded.AnalysedAt)
	assert.NotNil(t, decoded.Project)
	assert.Equal(t, event.Project.Key, decoded.Project.Key)
	assert.NotNil(t, decoded.QualityGate)
	assert.Equal(t, event.QualityGate.Status, decoded.QualityGate.Status)
	assert.NotNil(t, decoded.Task)
	assert.Equal(t, event.Task.ID, decoded.Task.ID)
}

func TestQueryOptions(t *testing.T) {
	// Test QueryOptions serialization
	options := QueryOptions{
		Limit:  100,
		Offset: 50,
		SortBy: "name",
		Order:  "asc",
	}

	// Test marshaling
	data, err := json.Marshal(options)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded QueryOptions
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, options.Limit, decoded.Limit)
	assert.Equal(t, options.Offset, decoded.Offset)
	assert.Equal(t, options.SortBy, decoded.SortBy)
	assert.Equal(t, options.Order, decoded.Order)
}

func TestHarnessQuery(t *testing.T) {
	// Test HarnessQuery serialization
	query := HarnessQuery{
		Type: HarnessQueryTypeCIBuild,
		ID:   "build-123",
	}

	// Test marshaling
	data, err := json.Marshal(query)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded HarnessQuery
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, query.Type, decoded.Type)
	assert.Equal(t, query.ID, decoded.ID)
}

func TestSonarQubeQuery(t *testing.T) {
	// Test SonarQubeQuery serialization
	query := SonarQubeQuery{
		Type:         SonarQubeQueryTypeProject,
		ProjectKey:   "org:project",
		Organization: "org",
		Severity:     "MAJOR",
		Status:       "OPEN",
		MetricKeys:   "bugs,code_smells,vulnerabilities",
	}

	// Test marshaling
	data, err := json.Marshal(query)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded SonarQubeQuery
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, query.Type, decoded.Type)
	assert.Equal(t, query.ProjectKey, decoded.ProjectKey)
	assert.Equal(t, query.Organization, decoded.Organization)
	assert.Equal(t, query.Severity, decoded.Severity)
	assert.Equal(t, query.Status, decoded.Status)
	assert.Equal(t, query.MetricKeys, decoded.MetricKeys)
}

func TestArtifactoryQuery(t *testing.T) {
	// Test ArtifactoryQuery serialization
	query := ArtifactoryQuery{
		Type:        ArtifactoryQueryTypeArtifact,
		RepoKey:     "libs-release",
		Path:        "org/example/app/1.0.0/app-1.0.0.jar",
		RepoType:    "local",
		PackageType: "maven",
		BuildName:   "app-build",
		BuildNumber: "123",
	}

	// Test marshaling
	data, err := json.Marshal(query)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded ArtifactoryQuery
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, query.Type, decoded.Type)
	assert.Equal(t, query.RepoKey, decoded.RepoKey)
	assert.Equal(t, query.Path, decoded.Path)
	assert.Equal(t, query.RepoType, decoded.RepoType)
	assert.Equal(t, query.PackageType, decoded.PackageType)
	assert.Equal(t, query.BuildName, decoded.BuildName)
	assert.Equal(t, query.BuildNumber, decoded.BuildNumber)
}

func TestXrayQuery(t *testing.T) {
	// Test XrayQuery serialization
	query := XrayQuery{
		Type:         XrayQueryTypeVulnerabilities,
		ArtifactPath: "libs-release/org/example/app/1.0.0/app-1.0.0.jar",
		CVE:          "CVE-2021-44228",
		LicenseID:    "MIT",
		BuildName:    "app-build",
		BuildNumber:  "123",
	}

	// Test marshaling
	data, err := json.Marshal(query)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test unmarshaling
	var decoded XrayQuery
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, query.Type, decoded.Type)
	assert.Equal(t, query.ArtifactPath, decoded.ArtifactPath)
	assert.Equal(t, query.CVE, decoded.CVE)
	assert.Equal(t, query.LicenseID, decoded.LicenseID)
	assert.Equal(t, query.BuildName, decoded.BuildName)
	assert.Equal(t, query.BuildNumber, decoded.BuildNumber)
}
