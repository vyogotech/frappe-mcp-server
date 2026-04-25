package server

import (
	"context"
	"log/slog"
	"strings"
)

// ReportSchema holds filter schema information for a report
type ReportSchema struct {
	ReportName       string
	RequiredFilters  []string
	OptionalFilters  []string
	DateRangeFields  []string              // The specific field names for date range
	CompanyFieldName string                // Usually "company", but could vary
	FilterDefaults   map[string]interface{} // Default values for filters
}

// GetReportSchema retrieves or infers the schema for a report
func (s *MCPServer) GetReportSchema(ctx context.Context, reportName string) (*ReportSchema, error) {
	// First, try to fetch from Frappe API
	filters, err := s.frappeClient.GetReportFilters(ctx, reportName)
	if err != nil {
		slog.Warn("Failed to fetch report filters from API, using fallback", "report_name", reportName, "error", err)
		return s.getHardcodedReportSchema(reportName), nil
	}

	if len(filters) == 0 {
		slog.Warn("No filters returned from API, using fallback", "report_name", reportName)
		return s.getHardcodedReportSchema(reportName), nil
	}

	schema := &ReportSchema{
		ReportName:      reportName,
		RequiredFilters: []string{},
		OptionalFilters: []string{},
		DateRangeFields: []string{},
		FilterDefaults:  make(map[string]interface{}),
	}

	// Parse filters
	for _, filter := range filters {
		fieldName := filter.FieldName
		
		// Identify date range fields
		if filter.FieldType == "Date" {
			schema.DateRangeFields = append(schema.DateRangeFields, fieldName)
		}

		// Identify company field
		if strings.Contains(strings.ToLower(fieldName), "company") {
			schema.CompanyFieldName = fieldName
		}

		// Store default values
		if filter.Default != nil {
			schema.FilterDefaults[fieldName] = filter.Default
		}

		// Categorize as required or optional
		if filter.Mandatory == 1 {
			schema.RequiredFilters = append(schema.RequiredFilters, fieldName)
		} else {
			schema.OptionalFilters = append(schema.OptionalFilters, fieldName)
		}
	}

	slog.Info("Retrieved report schema dynamically",
		"report_name", reportName,
		"required_filters", schema.RequiredFilters,
		"date_fields", schema.DateRangeFields,
		"defaults", schema.FilterDefaults)

	return schema, nil
}

// getHardcodedReportSchema provides fallback schema for common reports
// This should rarely be used - the API fetch should work for most cases
func (s *MCPServer) getHardcodedReportSchema(reportName string) *ReportSchema {
	// Normalize report name for matching
	normalized := strings.ToLower(strings.TrimSpace(reportName))

	// Financial reports (use period_start_date/period_end_date)
	if strings.Contains(normalized, "profit and loss") ||
		strings.Contains(normalized, "balance sheet") ||
		strings.Contains(normalized, "cash flow") {
		return &ReportSchema{
			ReportName:       reportName,
			RequiredFilters:  []string{"period_start_date", "period_end_date", "company", "periodicity"},
			OptionalFilters:  []string{"finance_book"},
			DateRangeFields:  []string{"period_start_date", "period_end_date"},
			CompanyFieldName: "company",
			FilterDefaults:   map[string]interface{}{"periodicity": "Monthly"},
		}
	}

	// Most other ERPNext reports use from_date/to_date
	return &ReportSchema{
		ReportName:       reportName,
		RequiredFilters:  []string{"from_date", "to_date"},
		OptionalFilters:  []string{"company"},
		DateRangeFields:  []string{"from_date", "to_date"},
		CompanyFieldName: "company",
		FilterDefaults:   map[string]interface{}{},
	}
}

// ValidateAndTransformFilters validates user-provided filters against schema and transforms them
func (s *ReportSchema) ValidateAndTransformFilters(userFilters map[string]interface{}) (map[string]interface{}, []string, error) {
	transformed := make(map[string]interface{})
	missing := []string{}

	// Copy provided filters
	for k, v := range userFilters {
		transformed[k] = v
	}

	// Smart date mapping: if user provided generic "from_date"/"to_date" but report needs period fields
	if len(s.DateRangeFields) >= 2 {
		expectedStart := s.DateRangeFields[0]
		expectedEnd := s.DateRangeFields[1]

		// If user said "from_date" but we need "period_start_date"
		if fromDate, hasFrom := transformed["from_date"]; hasFrom && expectedStart != "from_date" {
			transformed[expectedStart] = fromDate
			delete(transformed, "from_date")
			slog.Info("Mapped from_date to "+expectedStart, "value", fromDate)
		}

		// If user said "to_date" but we need "period_end_date"
		if toDate, hasTo := transformed["to_date"]; hasTo && expectedEnd != "to_date" {
			transformed[expectedEnd] = toDate
			delete(transformed, "to_date")
			slog.Info("Mapped to_date to "+expectedEnd, "value", toDate)
		}

		// Reverse mapping: if user said "period_start_date" but we need "from_date"
		if periodStart, hasPeriodStart := transformed["period_start_date"]; hasPeriodStart && expectedStart != "period_start_date" {
			transformed[expectedStart] = periodStart
			delete(transformed, "period_start_date")
			slog.Info("Mapped period_start_date to "+expectedStart, "value", periodStart)
		}

		if periodEnd, hasPeriodEnd := transformed["period_end_date"]; hasPeriodEnd && expectedEnd != "period_end_date" {
			transformed[expectedEnd] = periodEnd
			delete(transformed, "period_end_date")
			slog.Info("Mapped period_end_date to "+expectedEnd, "value", periodEnd)
		}
	}

	// Add default periodicity for financial reports if not provided
	if _, hasperiodicity := transformed["periodicity"]; !hasperiodicity {
		for _, reqFilter := range s.RequiredFilters {
			if reqFilter == "periodicity" {
				// Use schema default if available, otherwise use "Monthly"
				if defaultVal, hasDefault := s.FilterDefaults["periodicity"]; hasDefault {
					transformed["periodicity"] = defaultVal
					slog.Info("Added default periodicity from schema", "value", defaultVal)
				} else {
					transformed["periodicity"] = "Monthly"
					slog.Info("Added default periodicity", "value", "Monthly")
				}
				break
			}
		}
	}
	
	// Apply other defaults for missing required fields
	for _, reqFilter := range s.RequiredFilters {
		if _, exists := transformed[reqFilter]; !exists {
			if defaultVal, hasDefault := s.FilterDefaults[reqFilter]; hasDefault {
				transformed[reqFilter] = defaultVal
				slog.Info("Applied default value for required filter", "filter", reqFilter, "value", defaultVal)
			}
		}
	}

	// Check for missing required filters
	for _, reqFilter := range s.RequiredFilters {
		if _, exists := transformed[reqFilter]; !exists {
			missing = append(missing, reqFilter)
		}
	}

	return transformed, missing, nil
}

// GetDateFieldNames returns the date field names expected by this report
func (s *ReportSchema) GetDateFieldNames() (startField, endField string) {
	if len(s.DateRangeFields) >= 2 {
		return s.DateRangeFields[0], s.DateRangeFields[1]
	}
	// Default fallback
	return "from_date", "to_date"
}

