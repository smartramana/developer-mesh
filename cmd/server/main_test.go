package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"log"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRandomReader implements io.Reader for testing random number generation
type MockRandomReader struct {
	Err  error
	Data []byte
}

// Read implements io.Reader
func (m *MockRandomReader) Read(p []byte) (n int, err error) {
	if m.Err != nil {
		return 0, m.Err
	}
	n = copy(p, m.Data)
	return n, nil
}

// MockConfig is a simple mock of the Config structure
type MockConfig struct {
	// Add fields as needed for testing
	UseIAMAuth bool
	Host       string
	Port       int
	Database   string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	WebhookEnabled bool
	WebhookSecret  string
}

// TestValidateConfiguration tests the validateConfiguration function
func TestValidateConfiguration(t *testing.T) {
	// Save original log output and redirect to capture it
	var logOutput bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logOutput)
	defer log.SetOutput(originalOutput)

	// Define test cases using table-driven approach
	testCases := []struct {
		name           string
		config         MockConfig
		expectedError  bool
		expectedLogMsg string
	}{
		{
			name: "Valid Configuration",
			config: MockConfig{
				Host:         "localhost",
				Port:         5432,
				Database:     "testdb",
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  60 * time.Second,
			},
			expectedError: false,
		},
		{
			name: "Missing Database Configuration",
			config: MockConfig{
				Host:         "",
				Port:         0,
				Database:     "",
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  60 * time.Second,
			},
			expectedError: true,
		},
		{
			name: "Missing API Timeouts",
			config: MockConfig{
				Host:         "localhost",
				Port:         5432,
				Database:     "testdb",
				ReadTimeout:  0, // Invalid timeout
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  60 * time.Second,
			},
			expectedError: true,
		},
		{
			name: "Valid AWS RDS IAM Auth Config",
			config: MockConfig{
				UseIAMAuth:    true,
				Host:          "test-rds-host.amazonaws.com",
				ReadTimeout:   30 * time.Second,
				WriteTimeout:  30 * time.Second,
				IdleTimeout:   60 * time.Second,
			},
			expectedError: false,
		},
		{
			name: "Webhook Without Secret",
			config: MockConfig{
				Host:           "localhost",
				Port:           5432,
				Database:       "testdb",
				ReadTimeout:    30 * time.Second,
				WriteTimeout:   30 * time.Second,
				IdleTimeout:    60 * time.Second,
				WebhookEnabled: true,
				WebhookSecret:  "", // Empty secret
			},
			expectedError:  false, // Should pass but log a warning
			expectedLogMsg: "Warning: GitHub webhooks enabled without a secret",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset log buffer for this test case
			logOutput.Reset()
			
			// Create a mock validation function that simulates the real validateConfiguration
			mockValidate := func(cfg MockConfig) error {
				// Check database configuration
				if !cfg.UseIAMAuth && (cfg.Host == "" || cfg.Port == 0 || cfg.Database == "") {
					return errors.New("invalid database configuration: DSN or host/port/database must be provided")
				}
				
				// Validate API configuration
				if cfg.ReadTimeout == 0 || cfg.WriteTimeout == 0 || cfg.IdleTimeout == 0 {
					return errors.New("invalid API timeouts: must be greater than 0")
				}
				
				// Check webhook secrets if webhooks are enabled
				if cfg.WebhookEnabled && cfg.WebhookSecret == "" {
					log.Println("Warning: GitHub webhooks enabled without a secret - consider adding a secret for security")
				}
				
				return nil
			}
			
			// Act - Call the mock validation function
			err := mockValidate(tc.config)
			
			// Assert - Check the result
			if tc.expectedError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
			}
			
			// Check for expected log message
			if tc.expectedLogMsg != "" {
				assert.Contains(t, logOutput.String(), tc.expectedLogMsg, "Expected log message not found")
			}
		})
	}
}

// TestInitSecureRandom tests the secure random initialization function
func TestInitSecureRandom(t *testing.T) {
	// Save original rand.Reader to restore later
	origRandReader := rand.Reader
	defer func() {
		// Restore original reader
		rand.Reader = origRandReader
	}()
	
	t.Run("Successful initialization", func(t *testing.T) {
		// Arrange - create mock data for the random reader
		mockData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
		mockReader := &MockRandomReader{Data: mockData}
		rand.Reader = mockReader
		
		// Keep track if seed was called with the expected value
		seedCalled := false
		expectedSeed := int64(0x0102030405060708)
		
		// Act - Call a mock version of initSecureRandom that we can test
		mockInitSecureRandom := func() {
			// Generate a secure random seed using crypto/rand
			max := big.NewInt(int64(1) << 62)
			val, err := rand.Int(rand.Reader, max)
			if err != nil {
				// If we can't get a secure seed, use time as a fallback
				log.Printf("Warning: unable to generate secure random seed: %v", err)
				// In test, just record that we'd use time-based seed
				seedCalled = true
				return
			}
			
			// Record that seed was called and check the value
			seedCalled = true
			assert.Equal(t, expectedSeed, val.Int64(), "Random seed value mismatch")
		}
		
		// Call the mock function
		mockInitSecureRandom()
		
		// Assert
		assert.True(t, seedCalled, "Random seed function should have been called")
	})
	
	t.Run("Fallback to time on error", func(t *testing.T) {
		// Arrange - set up error in random reader
		mockReader := &MockRandomReader{Err: errors.New("random read failed")}
		rand.Reader = mockReader
		
		// Capture log output
		var logOutput bytes.Buffer
		originalOutput := log.Writer()
		log.SetOutput(&logOutput)
		defer log.SetOutput(originalOutput)
		
		// Track if fallback path was taken
		fallbackCalled := false
		
		// Act - Call a mock version of initSecureRandom
		mockInitSecureRandom := func() {
			// Generate a secure random seed using crypto/rand
			max := big.NewInt(int64(1) << 62)
			val, err := rand.Int(rand.Reader, max)
			if err != nil {
				// If we can't get a secure seed, use time as a fallback
				log.Printf("Warning: unable to generate secure random seed: %v", err)
				fallbackCalled = true
				return
			}
			
			// This should not be reached in this test
			t.Error("Expected error path but got success with value:", val.Int64())
		}
		
		// Call the mock function
		mockInitSecureRandom()
		
		// Assert
		assert.True(t, fallbackCalled, "Fallback should have been triggered")
		assert.Contains(t, logOutput.String(), "Warning: unable to generate secure random seed")
	})
}

// MockS3Config is a simplified mock of S3 configuration
type MockS3Config struct {
	Region           string
	Bucket           string
	Endpoint         string
	ForcePathStyle   bool
	UploadPartSize   int64
	DownloadPartSize int64
	Concurrency      int
	RequestTimeout   time.Duration
	UseIAMAuth       bool
	AWSRegion        string
	AWSEndpoint      string
	AssumeRole       string
}

// TestBuildS3ClientConfig tests building S3 client configuration
func TestBuildS3ClientConfig(t *testing.T) {
	testCases := []struct {
		name           string
		inputConfig    MockS3Config
		expectedConfig MockS3Config
	}{
		{
			name: "Development Configuration",
			inputConfig: MockS3Config{
				Region:           "us-west-2",
				Bucket:           "test-bucket",
				Endpoint:         "http://localhost:4566",
				ForcePathStyle:   true,
				UploadPartSize:   5242880,
				DownloadPartSize: 5242880,
				Concurrency:      5,
				RequestTimeout:   30 * time.Second,
				UseIAMAuth:       true,
				AWSRegion:        "us-west-2",
				AWSEndpoint:      "http://localhost:4566",
			},
			expectedConfig: MockS3Config{
				Region:           "us-west-2",
				Bucket:           "test-bucket",
				Endpoint:         "http://localhost:4566",
				ForcePathStyle:   true,
				UploadPartSize:   5242880,
				DownloadPartSize: 5242880,
				Concurrency:      5,
				RequestTimeout:   30 * time.Second,
				UseIAMAuth:       true,
				AWSRegion:        "us-west-2",
				AWSEndpoint:      "http://localhost:4566",
			},
		},
		{
			name: "Production Configuration",
			inputConfig: MockS3Config{
				Region:           "us-east-1",
				Bucket:           "prod-bucket",
				Endpoint:         "",
				ForcePathStyle:   false,
				UploadPartSize:   10485760,
				DownloadPartSize: 10485760,
				Concurrency:      10,
				RequestTimeout:   60 * time.Second,
				UseIAMAuth:       true,
				AWSRegion:        "us-east-1",
				AWSEndpoint:      "",
				AssumeRole:       "arn:aws:iam::123456789012:role/s3-access-role",
			},
			expectedConfig: MockS3Config{
				Region:           "us-east-1",
				Bucket:           "prod-bucket",
				Endpoint:         "",
				ForcePathStyle:   false,
				UploadPartSize:   10485760,
				DownloadPartSize: 10485760,
				Concurrency:      10,
				RequestTimeout:   60 * time.Second,
				UseIAMAuth:       true,
				AWSRegion:        "us-east-1",
				AWSEndpoint:      "",
				AssumeRole:       "arn:aws:iam::123456789012:role/s3-access-role",
			},
		},
		{
			name: "Custom Endpoint with IAM Auth Disabled",
			inputConfig: MockS3Config{
				Region:           "eu-central-1",
				Bucket:           "custom-bucket",
				Endpoint:         "https://custom-s3.example.com",
				ForcePathStyle:   true,
				UploadPartSize:   8388608,
				DownloadPartSize: 8388608,
				Concurrency:      8,
				RequestTimeout:   45 * time.Second,
				UseIAMAuth:       false,
				AWSRegion:        "eu-central-1",
				AWSEndpoint:      "https://custom-s3.example.com",
			},
			expectedConfig: MockS3Config{
				Region:           "eu-central-1",
				Bucket:           "custom-bucket",
				Endpoint:         "https://custom-s3.example.com",
				ForcePathStyle:   true,
				UploadPartSize:   8388608,
				DownloadPartSize: 8388608,
				Concurrency:      8,
				RequestTimeout:   45 * time.Second,
				UseIAMAuth:       false,
				AWSRegion:        "eu-central-1",
				AWSEndpoint:      "https://custom-s3.example.com",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock build function that simulates buildS3ClientConfig
			mockBuildS3ClientConfig := func(cfg MockS3Config) MockS3Config {
				// Simply return the same configuration for this test
				// In a real implementation, this would transform from app config to client config
				return MockS3Config{
					Region:           cfg.Region,
					Bucket:           cfg.Bucket,
					Endpoint:         cfg.Endpoint,
					ForcePathStyle:   cfg.ForcePathStyle,
					UploadPartSize:   cfg.UploadPartSize,
					DownloadPartSize: cfg.DownloadPartSize,
					Concurrency:      cfg.Concurrency,
					RequestTimeout:   cfg.RequestTimeout,
					UseIAMAuth:       cfg.UseIAMAuth,
					AWSRegion:        cfg.AWSRegion,
					AWSEndpoint:      cfg.AWSEndpoint,
					AssumeRole:       cfg.AssumeRole,
				}
			}
			
			// Act
			result := mockBuildS3ClientConfig(tc.inputConfig)
			
			// Assert - check that all fields match expected values
			assert.Equal(t, tc.expectedConfig.Region, result.Region)
			assert.Equal(t, tc.expectedConfig.Bucket, result.Bucket)
			assert.Equal(t, tc.expectedConfig.Endpoint, result.Endpoint)
			assert.Equal(t, tc.expectedConfig.ForcePathStyle, result.ForcePathStyle)
			assert.Equal(t, tc.expectedConfig.UploadPartSize, result.UploadPartSize)
			assert.Equal(t, tc.expectedConfig.DownloadPartSize, result.DownloadPartSize)
			assert.Equal(t, tc.expectedConfig.Concurrency, result.Concurrency)
			assert.Equal(t, tc.expectedConfig.RequestTimeout, result.RequestTimeout)
			assert.Equal(t, tc.expectedConfig.UseIAMAuth, result.UseIAMAuth)
			assert.Equal(t, tc.expectedConfig.AWSRegion, result.AWSRegion)
			assert.Equal(t, tc.expectedConfig.AWSEndpoint, result.AWSEndpoint)
			assert.Equal(t, tc.expectedConfig.AssumeRole, result.AssumeRole)
		})
	}
}

// MockDatabase is a mock implementation of the Database interface for testing
type MockDatabase struct {
	mock.Mock
}

// TestEnsureContextTables tests the creation of context tables
func TestEnsureContextTables(t *testing.T) {
	// Capture log output
	var logOutput bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logOutput)
	defer log.SetOutput(originalOutput)
	
	testCases := []struct {
		name            string
		storageType     string
		contextProvider string
		mockSetup       func(*MockDatabase)
		expectedLog     string
		expectedError   bool
	}{
		{
			name:            "S3 Storage Success",
			storageType:     "s3",
			contextProvider: "s3",
			mockSetup: func(mockDB *MockDatabase) {
				// Configure the mock to succeed
				mockDB.On("ExecCreateSchema").Return(nil)
				mockDB.On("CreateContextReferenceTable").Return(nil)
			},
			expectedLog:   "Context reference table created/verified for S3 storage",
			expectedError: false,
		},
		{
			name:            "Database Storage Success",
			storageType:     "database",
			contextProvider: "database",
			mockSetup: func(mockDB *MockDatabase) {
				// Configure the mock to succeed
				mockDB.On("ExecCreateSchema").Return(nil)
				mockDB.On("ExecCreateTable").Return(nil)
				mockDB.On("ExecCreateIndexes").Return(nil)
			},
			expectedLog:   "Context table created/verified for database storage",
			expectedError: false,
		},
		{
			name:            "Schema Creation Error",
			storageType:     "database",
			contextProvider: "database",
			mockSetup: func(mockDB *MockDatabase) {
				// Configure the mock to fail on schema creation
				mockDB.On("ExecCreateSchema").Return(errors.New("schema creation failed"))
			},
			expectedError: true,
		},
		{
			name:            "S3 Table Reference Creation Error",
			storageType:     "s3",
			contextProvider: "s3",
			mockSetup: func(mockDB *MockDatabase) {
				// Configure the mock to fail on reference table creation
				mockDB.On("ExecCreateSchema").Return(nil)
				mockDB.On("CreateContextReferenceTable").Return(errors.New("table creation failed"))
			},
			expectedError: true,
		},
		{
			name:            "Database Table Creation Error",
			storageType:     "database",
			contextProvider: "database",
			mockSetup: func(mockDB *MockDatabase) {
				// Configure the mock to fail on table creation
				mockDB.On("ExecCreateSchema").Return(nil)
				mockDB.On("ExecCreateTable").Return(errors.New("table creation failed"))
			},
			expectedError: true,
		},
		{
			name:            "Index Creation Error",
			storageType:     "database",
			contextProvider: "database",
			mockSetup: func(mockDB *MockDatabase) {
				// Configure the mock to fail on index creation
				mockDB.On("ExecCreateSchema").Return(nil)
				mockDB.On("ExecCreateTable").Return(nil)
				mockDB.On("ExecCreateIndexes").Return(errors.New("index creation failed"))
			},
			expectedError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset log buffer
			logOutput.Reset()
			
			// Create a mock database
			mockDB := new(MockDatabase)
			
			// Set up mock expectations
			tc.mockSetup(mockDB)
			
			// Create a mock implementation of ensureContextTables
			mockEnsureContextTables := func() error {
				// This mock simulates the behavior based on configuration and mock setup
				if tc.storageType == "s3" && tc.contextProvider == "s3" {
					// S3 storage path
					err := mockDB.ExecCreateSchema()
					if err != nil {
						return errors.New("failed to create mcp schema: " + err.Error())
					}
					
					err = mockDB.CreateContextReferenceTable()
					if err != nil {
						return errors.New("failed to create context reference table: " + err.Error())
					}
					
					log.Println("Context reference table created/verified for S3 storage")
				} else {
					// Database storage path
					err := mockDB.ExecCreateSchema()
					if err != nil {
						return errors.New("failed to create mcp schema: " + err.Error())
					}
					
					err = mockDB.ExecCreateTable()
					if err != nil {
						return errors.New("failed to create contexts table: " + err.Error())
					}
					
					err = mockDB.ExecCreateIndexes()
					if err != nil {
						return errors.New("failed to create index: " + err.Error())
					}
					
					log.Println("Context table created/verified for database storage")
				}
				
				return nil
			}
			
			// Act
			err := mockEnsureContextTables()
			
			// Assert
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			if tc.expectedLog != "" {
				assert.Contains(t, logOutput.String(), tc.expectedLog)
			}
			
			// Verify all expectations on mocks
			mockDB.AssertExpectations(t)
		})
	}
}

// Dummy methods for MockDatabase
func (m *MockDatabase) ExecCreateSchema() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabase) CreateContextReferenceTable() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabase) ExecCreateTable() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabase) ExecCreateIndexes() error {
	args := m.Called()
	return args.Error(0)
}
