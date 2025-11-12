package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateID(t *testing.T) {
	// Test ID generation
	id1 := GenerateID(8)
	id2 := GenerateID(8)

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Equal(t, 8, len(id1))

	// Test different lengths
	id16 := GenerateID(16)
	assert.Equal(t, 16, len(id16))
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal string",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "string with HTML tags",
			input:    "<p>Hello <b>World</b></p>",
			expected: "Hello World",
		},
		{
			name:     "string with script tags",
			input:    "Hello <script>alert('xss')</script> World",
			expected: "Hello  World",
		},
		{
			name:     "string with whitespace",
			input:    "  Hello World  ",
			expected: "Hello World",
		},
		{
			name:     "mixed HTML and whitespace",
			input:    "  <div>Hello <span>World</span></div>  ",
			expected: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{
			name:     "valid email",
			email:    "test@example.com",
			expected: true,
		},
		{
			name:     "valid email with subdomain",
			email:    "user@mail.example.com",
			expected: true,
		},
		{
			name:     "valid email with numbers",
			email:    "user123@example123.com",
			expected: true,
		},
		{
			name:     "invalid email - no @",
			email:    "testexample.com",
			expected: false,
		},
		{
			name:     "invalid email - no domain",
			email:    "test@",
			expected: false,
		},
		{
			name:     "invalid email - no TLD",
			email:    "test@example",
			expected: false,
		},
		{
			name:     "invalid email - multiple @",
			email:    "test@@example.com",
			expected: false,
		},
		{
			name:     "empty email",
			email:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateEmail(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		currency string
		expected string
	}{
		{
			name:     "USD amount",
			amount:   1234.56,
			currency: "USD",
			expected: "1234.56 USD",
		},
		{
			name:     "EUR amount",
			amount:   999.99,
			currency: "EUR",
			expected: "999.99 EUR",
		},
		{
			name:     "zero amount",
			amount:   0.0,
			currency: "USD",
			expected: "0.00 USD",
		},
		{
			name:     "negative amount",
			amount:   -500.25,
			currency: "USD",
			expected: "-500.25 USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCurrency(tt.amount, tt.currency)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculatePercentage(t *testing.T) {
	tests := []struct {
		name     string
		part     float64
		total    float64
		expected float64
	}{
		{
			name:     "normal calculation",
			part:     25.0,
			total:    100.0,
			expected: 25.0,
		},
		{
			name:     "half percentage",
			part:     50.0,
			total:    100.0,
			expected: 50.0,
		},
		{
			name:     "decimal result",
			part:     1.0,
			total:    3.0,
			expected: 33.33333333333333,
		},
		{
			name:     "zero total",
			part:     10.0,
			total:    0.0,
			expected: 0.0,
		},
		{
			name:     "zero part",
			part:     0.0,
			total:    100.0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculatePercentage(tt.part, tt.total)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		length   int
		expected string
	}{
		{
			name:     "string shorter than length",
			str:      "Hello",
			length:   10,
			expected: "Hello",
		},
		{
			name:     "string equal to length",
			str:      "Hello",
			length:   5,
			expected: "Hello",
		},
		{
			name:     "string longer than length",
			str:      "Hello World",
			length:   8,
			expected: "Hello...",
		},
		{
			name:     "very short length",
			str:      "Hello World",
			length:   3,
			expected: "Hel",
		},
		{
			name:     "length of 1",
			str:      "Hello",
			length:   1,
			expected: "H",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.str, tt.length)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	tests := []struct {
		name     string
		item     string
		expected bool
	}{
		{
			name:     "item exists",
			item:     "banana",
			expected: true,
		},
		{
			name:     "item does not exist",
			item:     "orange",
			expected: false,
		},
		{
			name:     "empty item",
			item:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsString(slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveEmptyStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no empty strings",
			input:    []string{"apple", "banana", "cherry"},
			expected: []string{"apple", "banana", "cherry"},
		},
		{
			name:     "with empty strings",
			input:    []string{"apple", "", "banana", "", "cherry"},
			expected: []string{"apple", "banana", "cherry"},
		},
		{
			name:     "with whitespace strings",
			input:    []string{"apple", "  ", "banana", "\t", "cherry"},
			expected: []string{"apple", "banana", "cherry"},
		},
		{
			name:     "all empty",
			input:    []string{"", "  ", "\t"},
			expected: []string{},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveEmptyStrings(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDateString(t *testing.T) {
	tests := []struct {
		name        string
		dateStr     string
		expectError bool
	}{
		{
			name:        "ISO date",
			dateStr:     "2024-01-15",
			expectError: false,
		},
		{
			name:        "ISO datetime with Z",
			dateStr:     "2024-01-15T10:30:00Z",
			expectError: false,
		},
		{
			name:        "ISO datetime with milliseconds",
			dateStr:     "2024-01-15T10:30:00.123Z",
			expectError: false,
		},
		{
			name:        "datetime without timezone",
			dateStr:     "2024-01-15 10:30:00",
			expectError: false,
		},
		{
			name:        "US format",
			dateStr:     "01/15/2024",
			expectError: false,
		},
		{
			name:        "US format with dashes",
			dateStr:     "01-15-2024",
			expectError: false,
		},
		{
			name:        "invalid format",
			dateStr:     "not-a-date",
			expectError: true,
		},
		{
			name:        "empty string",
			dateStr:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDateString(tt.dateStr)
			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, result.IsZero())
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}
}

func TestCalculateBusinessDays(t *testing.T) {
	// Test different scenarios
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // Monday
	end := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)   // Friday

	// Monday to Friday (inclusive) = 5 business days
	result := CalculateBusinessDays(start, end)
	assert.Equal(t, 5, result)

	// Same day
	result = CalculateBusinessDays(start, start)
	assert.Equal(t, 1, result)

	// Reverse order should give same result
	result = CalculateBusinessDays(end, start)
	assert.Equal(t, 5, result)

	// Include weekend
	weekendEnd := time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC) // Sunday
	result = CalculateBusinessDays(start, weekendEnd)
	assert.Equal(t, 5, result) // Should still be 5 (excludes weekend)
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "30 seconds",
		},
		{
			name:     "minutes",
			duration: 2*time.Minute + 30*time.Second,
			expected: "2.5 minutes",
		},
		{
			name:     "hours",
			duration: 3*time.Hour + 30*time.Minute,
			expected: "3.5 hours",
		},
		{
			name:     "days",
			duration: 2*24*time.Hour + 12*time.Hour,
			expected: "2.5 days",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeMaps(t *testing.T) {
	map1 := map[string]interface{}{
		"a": 1,
		"b": 2,
	}

	map2 := map[string]interface{}{
		"c": 3,
		"d": 4,
	}

	map3 := map[string]interface{}{
		"b": 5, // Should overwrite map1["b"]
		"e": 6,
	}

	result := MergeMaps(map1, map2, map3)

	expected := map[string]interface{}{
		"a": 1,
		"b": 5, // Overwritten by map3
		"c": 3,
		"d": 4,
		"e": 6,
	}

	assert.Equal(t, expected, result)
}

func BenchmarkGenerateID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateID(16)
	}
}

func BenchmarkSanitizeString(b *testing.B) {
	input := "<p>Hello <script>alert('test')</script> <b>World</b></p>"
	for i := 0; i < b.N; i++ {
		SanitizeString(input)
	}
}

func BenchmarkValidateEmail(b *testing.B) {
	email := "test.user@example.com"
	for i := 0; i < b.N; i++ {
		ValidateEmail(email)
	}
}

func BenchmarkCalculatePercentage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculatePercentage(25.0, 100.0)
	}
}
