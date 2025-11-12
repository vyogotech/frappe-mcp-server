package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// GenerateID generates a random ID string
func GenerateID(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// SanitizeString removes potentially harmful characters from strings
func SanitizeString(input string) string {
	// First remove script content (case insensitive)
	re := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	cleaned := re.ReplaceAllString(input, "")

	// Remove HTML tags
	re = regexp.MustCompile(`<[^>]*>`)
	cleaned = re.ReplaceAllString(cleaned, "")

	// Trim whitespace
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// ValidateEmail validates email format
func ValidateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// FormatCurrency formats a float64 as currency
func FormatCurrency(amount float64, currency string) string {
	return fmt.Sprintf("%.2f %s", amount, currency)
}

// CalculatePercentage calculates percentage with proper handling of zero values
func CalculatePercentage(part, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (part / total) * 100
}

// TruncateString truncates a string to specified length with ellipsis
func TruncateString(str string, length int) string {
	if len(str) <= length {
		return str
	}
	if length <= 3 {
		return str[:length]
	}
	return str[:length-3] + "..."
}

// ContainsString checks if a string slice contains a specific string
func ContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RemoveEmptyStrings removes empty strings from a slice
func RemoveEmptyStrings(slice []string) []string {
	var result []string
	for _, s := range slice {
		if strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}

// ParseDateString parses various date string formats
func ParseDateString(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
		"01/02/2006",
		"01-02-2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// CalculateBusinessDays calculates business days between two dates
func CalculateBusinessDays(start, end time.Time) int {
	if start.After(end) {
		start, end = end, start
	}

	days := 0
	for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 0, 1) {
		weekday := d.Weekday()
		if weekday != time.Saturday && weekday != time.Sunday {
			days++
		}
	}

	return days
}

// FormatDuration formats a duration into human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}

// MergeMaps merges multiple maps into one
func MergeMaps(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
