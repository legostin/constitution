package check

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/legostin/constitution/pkg/types"
)

// DetectSecrets integrates Yelp detect-secrets for secret scanning.
// It dynamically generates a baseline config from YAML params and
// calls `detect-secrets scan --string` for each line of content.
type DetectSecrets struct {
	binary         string
	scanField      string
	scanMode       string // "line" | "content"
	baselinePath   string
	excludeSecrets []string
	excludeLines   []string
	timeout        time.Duration
}

func (d *DetectSecrets) Name() string { return "secret_yelp" }

func (d *DetectSecrets) Init(params map[string]interface{}) error {
	// Binary path
	d.binary = "detect-secrets"
	if b, ok := params["binary"].(string); ok && b != "" {
		d.binary = b
	}

	// Verify binary exists
	if _, err := exec.LookPath(d.binary); err != nil {
		return fmt.Errorf("secret_yelp: binary %q not found in PATH: %w", d.binary, err)
	}

	// Scan field
	d.scanField = "content"
	if sf, ok := params["scan_field"].(string); ok && sf != "" {
		d.scanField = sf
	}

	// Scan mode
	d.scanMode = "content"
	if sm, ok := params["scan_mode"].(string); ok && sm != "" {
		d.scanMode = sm
	}

	// Timeout
	d.timeout = 10 * time.Second
	if t, ok := params["timeout"]; ok {
		switch v := t.(type) {
		case int:
			d.timeout = time.Duration(v) * time.Millisecond
		case float64:
			d.timeout = time.Duration(v) * time.Millisecond
		}
	}

	// Exclude patterns
	if es, ok := params["exclude_secrets"]; ok {
		d.excludeSecrets, _ = toStringSlice(es)
	}
	if el, ok := params["exclude_lines"]; ok {
		d.excludeLines, _ = toStringSlice(el)
	}

	// Generate baseline config only if plugins or filters are specified
	hasPlugins := params["plugins"] != nil
	hasFilters := params["filters"] != nil
	if hasPlugins || hasFilters {
		baseline, err := d.generateBaseline(params)
		if err != nil {
			return fmt.Errorf("secret_yelp: failed to generate baseline: %w", err)
		}
		hash := sha256.Sum256(baseline)
		d.baselinePath = filepath.Join(os.TempDir(), fmt.Sprintf("constitution-ds-%x.json", hash[:8]))
		if err := os.WriteFile(d.baselinePath, baseline, 0o644); err != nil {
			return fmt.Errorf("secret_yelp: failed to write baseline: %w", err)
		}
	}

	return nil
}

func (d *DetectSecrets) Execute(ctx context.Context, input *types.HookInput) (*types.CheckResult, error) {
	content, err := extractContent(input, d.scanField)
	if err != nil || content == "" {
		return &types.CheckResult{Passed: true}, nil
	}

	switch d.scanMode {
	case "content":
		// Scan each non-empty line — detect-secrets file scan applies
		// aggressive filters that miss some secrets, so --string is more reliable
		return d.scanLines(ctx, content)
	default:
		return d.scanLines(ctx, content)
	}
}

// scanLines scans each line individually with `detect-secrets scan --string`.
func (d *DetectSecrets) scanLines(ctx context.Context, content string) (*types.CheckResult, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if d.isLineExcluded(line) {
			continue
		}

		detectors, err := d.scanString(ctx, line)
		if err != nil {
			continue // Don't block on scan errors
		}
		if len(detectors) > 0 {
			return &types.CheckResult{
				Passed:  false,
				Message: fmt.Sprintf("Secret detected by %s (line %d)", strings.Join(detectors, ", "), lineNum),
				Details: map[string]string{
					"detectors":   strings.Join(detectors, ", "),
					"line_number": fmt.Sprintf("%d", lineNum),
				},
			}, nil
		}
	}
	return &types.CheckResult{Passed: true, Message: "no secrets detected (detect-secrets)"}, nil
}

// scanContent writes content to a temp file and scans it with `detect-secrets scan`.
func (d *DetectSecrets) scanContent(ctx context.Context, content string) (*types.CheckResult, error) {
	// Use .py extension — detect-secrets skips certain file types
	tmpFile, err := os.CreateTemp("", "constitution-scan-*.py")
	if err != nil {
		return &types.CheckResult{Passed: true}, nil
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return &types.CheckResult{Passed: true}, nil
	}
	tmpFile.Close()

	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	args := []string{"scan", tmpFile.Name()}
	if d.baselinePath != "" {
		args = append(args, "--baseline", d.baselinePath)
	}
	for _, es := range d.excludeSecrets {
		args = append(args, "--exclude-secrets", es)
	}
	for _, el := range d.excludeLines {
		args = append(args, "--exclude-lines", el)
	}

	cmd := exec.CommandContext(ctx, d.binary, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return &types.CheckResult{Passed: true}, nil
	}

	// Parse baseline JSON output
	var baseline struct {
		Results map[string][]struct {
			Type       string `json:"type"`
			LineNumber int    `json:"line_number"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &baseline); err != nil {
		return &types.CheckResult{Passed: true}, nil
	}

	// Collect all detections
	var detections []string
	for _, secrets := range baseline.Results {
		for _, s := range secrets {
			detections = append(detections, fmt.Sprintf("%s (line %d)", s.Type, s.LineNumber))
		}
	}

	if len(detections) > 0 {
		return &types.CheckResult{
			Passed:  false,
			Message: fmt.Sprintf("Secrets detected: %s", strings.Join(detections, "; ")),
			Details: map[string]string{
				"detections": strings.Join(detections, "; "),
				"count":      fmt.Sprintf("%d", len(detections)),
			},
		}, nil
	}

	return &types.CheckResult{Passed: true, Message: "no secrets detected (detect-secrets)"}, nil
}

// scanString calls `detect-secrets scan --string` and returns detector names that matched.
func (d *DetectSecrets) scanString(ctx context.Context, s string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	args := []string{"scan", "--string", s}
	if d.baselinePath != "" {
		args = append(args, "--baseline", d.baselinePath)
	}

	cmd := exec.CommandContext(ctx, d.binary, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return parseStringOutput(stdout.String()), nil
}

// parseStringOutput parses the text output of `detect-secrets scan --string`.
// Format: "DetectorName          : True  (unverified)" or "DetectorName          : False"
func parseStringOutput(output string) []string {
	var matched []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if strings.HasPrefix(value, "True") {
			matched = append(matched, name)
		}
	}
	return matched
}

func (d *DetectSecrets) isLineExcluded(line string) bool {
	for _, pattern := range d.excludeLines {
		if strings.Contains(line, pattern) {
			return true
		}
	}
	return false
}

// generateBaseline builds the detect-secrets baseline JSON from YAML params.
func (d *DetectSecrets) generateBaseline(params map[string]interface{}) ([]byte, error) {
	baseline := map[string]interface{}{
		"version":    "1.5.0",
		"results":    map[string]interface{}{},
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}

	// Plugins
	var pluginsUsed []map[string]interface{}
	if rawPlugins, ok := params["plugins"]; ok {
		plugins, err := toSliceOfMaps(rawPlugins)
		if err != nil {
			return nil, fmt.Errorf("invalid plugins config: %w", err)
		}
		pluginsUsed = plugins
	}
	// If no plugins specified, don't include plugins_used — detect-secrets uses all by default
	if len(pluginsUsed) > 0 {
		baseline["plugins_used"] = pluginsUsed
	}

	// Filters
	var filtersUsed []map[string]interface{}
	if rawFilters, ok := params["filters"]; ok {
		filters, err := toSliceOfMaps(rawFilters)
		if err != nil {
			return nil, fmt.Errorf("invalid filters config: %w", err)
		}
		filtersUsed = filters
	}
	if len(filtersUsed) > 0 {
		baseline["filters_used"] = filtersUsed
	}

	return json.MarshalIndent(baseline, "", "  ")
}
