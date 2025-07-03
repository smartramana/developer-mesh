package reporting

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TestResult represents a single test result
type TestResult struct {
	Name        string
	Suite       string
	Status      TestStatus
	Duration    time.Duration
	StartTime   time.Time
	EndTime     time.Time
	Message     string
	Error       error
	Logs        []LogEntry
	Metrics     map[string]interface{}
	Screenshots []string
	Artifacts   []string
}

// TestStatus represents the status of a test
type TestStatus string

const (
	TestStatusPassed  TestStatus = "passed"
	TestStatusFailed  TestStatus = "failed"
	TestStatusSkipped TestStatus = "skipped"
	TestStatusPending TestStatus = "pending"
)

// LogEntry represents a log entry during test execution
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Data      map[string]interface{}
}

// TestSuite represents a collection of tests
type TestSuite struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Tests     []TestResult
	Summary   TestSummary
}

// TestSummary provides summary statistics
type TestSummary struct {
	Total     int
	Passed    int
	Failed    int
	Skipped   int
	Pending   int
	Duration  time.Duration
	StartTime time.Time
	EndTime   time.Time
}

// Reporter handles test reporting
type Reporter struct {
	suites     []TestSuite
	currentSuite *TestSuite
	mu         sync.RWMutex
	outputDir  string
	formats    []string
}

// NewReporter creates a new test reporter
func NewReporter(outputDir string, formats []string) *Reporter {
	if outputDir == "" {
		outputDir = "test-results"
	}
	
	if len(formats) == 0 {
		formats = []string{"json", "junit", "html"}
	}
	
	return &Reporter{
		suites:    make([]TestSuite, 0),
		outputDir: outputDir,
		formats:   formats,
	}
}

// StartSuite starts a new test suite
func (r *Reporter) StartSuite(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.currentSuite = &TestSuite{
		Name:      name,
		StartTime: time.Now(),
		Tests:     make([]TestResult, 0),
	}
}

// EndSuite ends the current test suite
func (r *Reporter) EndSuite() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.currentSuite == nil {
		return
	}
	
	r.currentSuite.EndTime = time.Now()
	r.currentSuite.Summary = r.calculateSummary(r.currentSuite.Tests)
	r.suites = append(r.suites, *r.currentSuite)
	r.currentSuite = nil
}

// RecordTest records a test result
func (r *Reporter) RecordTest(result TestResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.currentSuite != nil {
		r.currentSuite.Tests = append(r.currentSuite.Tests, result)
	}
}

// calculateSummary calculates summary statistics
func (r *Reporter) calculateSummary(tests []TestResult) TestSummary {
	summary := TestSummary{
		Total: len(tests),
	}
	
	if len(tests) == 0 {
		return summary
	}
	
	summary.StartTime = tests[0].StartTime
	summary.EndTime = tests[len(tests)-1].EndTime
	
	for _, test := range tests {
		summary.Duration += test.Duration
		
		switch test.Status {
		case TestStatusPassed:
			summary.Passed++
		case TestStatusFailed:
			summary.Failed++
		case TestStatusSkipped:
			summary.Skipped++
		case TestStatusPending:
			summary.Pending++
		}
		
		if test.StartTime.Before(summary.StartTime) {
			summary.StartTime = test.StartTime
		}
		if test.EndTime.After(summary.EndTime) {
			summary.EndTime = test.EndTime
		}
	}
	
	return summary
}

// GenerateReports generates reports in all configured formats
func (r *Reporter) GenerateReports() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Create output directory
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Generate reports in each format
	for _, format := range r.formats {
		switch format {
		case "json":
			if err := r.generateJSONReport(); err != nil {
				return fmt.Errorf("failed to generate JSON report: %w", err)
			}
		case "junit":
			if err := r.generateJUnitReport(); err != nil {
				return fmt.Errorf("failed to generate JUnit report: %w", err)
			}
		case "html":
			if err := r.generateHTMLReport(); err != nil {
				return fmt.Errorf("failed to generate HTML report: %w", err)
			}
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}
	}
	
	return nil
}

// generateJSONReport generates a JSON report
func (r *Reporter) generateJSONReport() error {
	filename := filepath.Join(r.outputDir, "report.json")
	
	data, err := json.MarshalIndent(r.suites, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(filename, data, 0644)
}

// generateJUnitReport generates a JUnit XML report
func (r *Reporter) generateJUnitReport() error {
	filename := filepath.Join(r.outputDir, "junit.xml")
	
	type JUnitTestCase struct {
		XMLName   xml.Name `xml:"testcase"`
		Name      string   `xml:"name,attr"`
		ClassName string   `xml:"classname,attr"`
		Time      float64  `xml:"time,attr"`
		Failure   *string  `xml:"failure,omitempty"`
		Skipped   *string  `xml:"skipped,omitempty"`
	}
	
	type JUnitTestSuite struct {
		XMLName   xml.Name        `xml:"testsuite"`
		Name      string          `xml:"name,attr"`
		Tests     int             `xml:"tests,attr"`
		Failures  int             `xml:"failures,attr"`
		Skipped   int             `xml:"skipped,attr"`
		Time      float64         `xml:"time,attr"`
		TestCases []JUnitTestCase `xml:"testcase"`
	}
	
	type JUnitTestSuites struct {
		XMLName xml.Name         `xml:"testsuites"`
		Suites  []JUnitTestSuite `xml:"testsuite"`
	}
	
	junitSuites := JUnitTestSuites{
		Suites: make([]JUnitTestSuite, 0, len(r.suites)),
	}
	
	for _, suite := range r.suites {
		junitSuite := JUnitTestSuite{
			Name:      suite.Name,
			Tests:     suite.Summary.Total,
			Failures:  suite.Summary.Failed,
			Skipped:   suite.Summary.Skipped,
			Time:      suite.Summary.Duration.Seconds(),
			TestCases: make([]JUnitTestCase, 0, len(suite.Tests)),
		}
		
		for _, test := range suite.Tests {
			tc := JUnitTestCase{
				Name:      test.Name,
				ClassName: suite.Name,
				Time:      test.Duration.Seconds(),
			}
			
			switch test.Status {
			case TestStatusFailed:
				msg := test.Message
				if test.Error != nil {
					msg = fmt.Sprintf("%s: %v", msg, test.Error)
				}
				tc.Failure = &msg
			case TestStatusSkipped:
				msg := "Test skipped"
				tc.Skipped = &msg
			}
			
			junitSuite.TestCases = append(junitSuite.TestCases, tc)
		}
		
		junitSuites.Suites = append(junitSuites.Suites, junitSuite)
	}
	
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	
	if _, err := file.WriteString(xml.Header); err != nil {
		return err
	}
	
	return encoder.Encode(junitSuites)
}

// generateHTMLReport generates an HTML report
func (r *Reporter) generateHTMLReport() error {
	filename := filepath.Join(r.outputDir, "report.html")
	
	htmlTemplate := `<!DOCTYPE html>
<html>
<head>
    <title>E2E Test Report</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1, h2 {
            color: #333;
        }
        .summary {
            display: flex;
            gap: 20px;
            margin: 20px 0;
        }
        .summary-card {
            flex: 1;
            padding: 15px;
            border-radius: 4px;
            text-align: center;
        }
        .passed { background-color: #d4edda; color: #155724; }
        .failed { background-color: #f8d7da; color: #721c24; }
        .skipped { background-color: #fff3cd; color: #856404; }
        .pending { background-color: #d1ecf1; color: #0c5460; }
        
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background-color: #f8f9fa;
            font-weight: bold;
        }
        tr:hover {
            background-color: #f5f5f5;
        }
        .status-icon {
            width: 20px;
            height: 20px;
            display: inline-block;
            border-radius: 50%;
            margin-right: 5px;
        }
        .status-passed { background-color: #28a745; }
        .status-failed { background-color: #dc3545; }
        .status-skipped { background-color: #ffc107; }
        .status-pending { background-color: #17a2b8; }
    </style>
</head>
<body>
    <div class="container">
        <h1>E2E Test Report</h1>
        <p>Generated: {{.GeneratedAt}}</p>
        
        {{range .Suites}}
        <h2>{{.Name}}</h2>
        <div class="summary">
            <div class="summary-card passed">
                <h3>{{.Summary.Passed}}</h3>
                <p>Passed</p>
            </div>
            <div class="summary-card failed">
                <h3>{{.Summary.Failed}}</h3>
                <p>Failed</p>
            </div>
            <div class="summary-card skipped">
                <h3>{{.Summary.Skipped}}</h3>
                <p>Skipped</p>
            </div>
            <div class="summary-card pending">
                <h3>{{.Summary.Pending}}</h3>
                <p>Pending</p>
            </div>
        </div>
        
        <table>
            <thead>
                <tr>
                    <th>Test Name</th>
                    <th>Status</th>
                    <th>Duration</th>
                    <th>Message</th>
                </tr>
            </thead>
            <tbody>
                {{range .Tests}}
                <tr>
                    <td>{{.Name}}</td>
                    <td>
                        <span class="status-icon status-{{.Status}}"></span>
                        {{.Status}}
                    </td>
                    <td>{{.Duration}}</td>
                    <td>{{.Message}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{end}}
    </div>
</body>
</html>`
	
	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return err
	}
	
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	
	data := struct {
		Suites      []TestSuite
		GeneratedAt string
	}{
		Suites:      r.suites,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	
	return tmpl.Execute(file, data)
}

// StreamingReporter provides real-time test reporting
type StreamingReporter struct {
	*Reporter
	writers []io.Writer
}

// NewStreamingReporter creates a new streaming reporter
func NewStreamingReporter(outputDir string, formats []string, writers ...io.Writer) *StreamingReporter {
	if len(writers) == 0 {
		writers = []io.Writer{os.Stdout}
	}
	
	return &StreamingReporter{
		Reporter: NewReporter(outputDir, formats),
		writers:  writers,
	}
}

// LogTest logs a test in real-time
func (sr *StreamingReporter) LogTest(result TestResult) {
	// Record in parent reporter
	sr.RecordTest(result)
	
	// Stream to writers
	statusSymbol := "✓"
	statusColor := "\033[32m" // green
	
	switch result.Status {
	case TestStatusFailed:
		statusSymbol = "✗"
		statusColor = "\033[31m" // red
	case TestStatusSkipped:
		statusSymbol = "⊘"
		statusColor = "\033[33m" // yellow
	case TestStatusPending:
		statusSymbol = "◌"
		statusColor = "\033[36m" // cyan
	}
	
	message := fmt.Sprintf("%s%s\033[0m %s (%v)\n", 
		statusColor, statusSymbol, result.Name, result.Duration)
	
	for _, w := range sr.writers {
		_, _ = fmt.Fprint(w, message)
	}
	
	// Log error details if failed
	if result.Status == TestStatusFailed && result.Error != nil {
		errorMsg := fmt.Sprintf("  Error: %v\n", result.Error)
		for _, w := range sr.writers {
			_, _ = fmt.Fprint(w, errorMsg)
		}
	}
}

// PrintSummary prints a summary of all tests
func (sr *StreamingReporter) PrintSummary() {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	
	for _, w := range sr.writers {
		_, _ = fmt.Fprintln(w, strings.Repeat("=", 80))
		_, _ = fmt.Fprintln(w, "TEST SUMMARY")
		_, _ = fmt.Fprintln(w, strings.Repeat("=", 80))
		
		totalSummary := TestSummary{}
		
		for _, suite := range sr.suites {
			_, _ = fmt.Fprintf(w, "\nSuite: %s\n", suite.Name)
			_, _ = fmt.Fprintf(w, "  Total:   %d\n", suite.Summary.Total)
			_, _ = fmt.Fprintf(w, "  Passed:  %d\n", suite.Summary.Passed)
			_, _ = fmt.Fprintf(w, "  Failed:  %d\n", suite.Summary.Failed)
			_, _ = fmt.Fprintf(w, "  Skipped: %d\n", suite.Summary.Skipped)
			_, _ = fmt.Fprintf(w, "  Duration: %v\n", suite.Summary.Duration)
			
			totalSummary.Total += suite.Summary.Total
			totalSummary.Passed += suite.Summary.Passed
			totalSummary.Failed += suite.Summary.Failed
			totalSummary.Skipped += suite.Summary.Skipped
			totalSummary.Duration += suite.Summary.Duration
		}
		
		_, _ = fmt.Fprintln(w, strings.Repeat("-", 80))
		_, _ = fmt.Fprintf(w, "TOTAL: %d tests, %d passed, %d failed, %d skipped\n",
			totalSummary.Total, totalSummary.Passed, totalSummary.Failed, totalSummary.Skipped)
		_, _ = fmt.Fprintf(w, "Duration: %v\n", totalSummary.Duration)
		_, _ = fmt.Fprintln(w, strings.Repeat("=", 80))
	}
}