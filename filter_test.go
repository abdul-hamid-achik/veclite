package veclite

import (
	"testing"
	"time"
)

func newTestRecord(id uint64, payload map[string]any) *Record {
	return &Record{
		ID:        id,
		Vector:    []float32{1, 2, 3},
		Payload:   payload,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestEqualFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    any
		payload  map[string]any
		expected bool
	}{
		{
			name:     "string match",
			key:      "lang",
			value:    "go",
			payload:  map[string]any{"lang": "go"},
			expected: true,
		},
		{
			name:     "string mismatch",
			key:      "lang",
			value:    "go",
			payload:  map[string]any{"lang": "python"},
			expected: false,
		},
		{
			name:     "int match",
			key:      "line",
			value:    42,
			payload:  map[string]any{"line": 42},
			expected: true,
		},
		{
			name:     "missing key",
			key:      "missing",
			value:    "value",
			payload:  map[string]any{"other": "value"},
			expected: false,
		},
		{
			name:     "nil payload",
			key:      "key",
			value:    "value",
			payload:  nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := Equal(tt.key, tt.value)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("Equal(%q, %v).Match() = %v, want %v", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestNotEqualFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    any
		payload  map[string]any
		expected bool
	}{
		{
			name:     "not equal",
			key:      "lang",
			value:    "python",
			payload:  map[string]any{"lang": "go"},
			expected: true,
		},
		{
			name:     "equal",
			key:      "lang",
			value:    "go",
			payload:  map[string]any{"lang": "go"},
			expected: false,
		},
		{
			name:     "missing key",
			key:      "missing",
			value:    "value",
			payload:  map[string]any{"other": "value"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NotEqual(tt.key, tt.value)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("NotEqual(%q, %v).Match() = %v, want %v", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestInFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		values   []any
		payload  map[string]any
		expected bool
	}{
		{
			name:     "in list",
			key:      "lang",
			values:   []any{"go", "python", "rust"},
			payload:  map[string]any{"lang": "go"},
			expected: true,
		},
		{
			name:     "not in list",
			key:      "lang",
			values:   []any{"java", "python"},
			payload:  map[string]any{"lang": "go"},
			expected: false,
		},
		{
			name:     "missing key",
			key:      "missing",
			values:   []any{"a", "b"},
			payload:  map[string]any{"other": "value"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := In(tt.key, tt.values...)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("In(%q, %v).Match() = %v, want %v", tt.key, tt.values, result, tt.expected)
			}
		})
	}
}

func TestNotInFilter(t *testing.T) {
	filter := NotIn("lang", "java", "python")

	goRecord := newTestRecord(1, map[string]any{"lang": "go"})
	if !filter.Match(goRecord) {
		t.Error("NotIn should match 'go' when excluding java/python")
	}

	javaRecord := newTestRecord(2, map[string]any{"lang": "java"})
	if filter.Match(javaRecord) {
		t.Error("NotIn should not match 'java'")
	}
}

func TestGlobFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		pattern  string
		payload  map[string]any
		expected bool
	}{
		{
			name:     "match extension",
			key:      "file",
			pattern:  "*.go",
			payload:  map[string]any{"file": "main.go"},
			expected: true,
		},
		{
			name:     "no match",
			key:      "file",
			pattern:  "*.go",
			payload:  map[string]any{"file": "main.py"},
			expected: false,
		},
		{
			name:     "match prefix",
			key:      "file",
			pattern:  "test_*",
			payload:  map[string]any{"file": "test_main.go"},
			expected: true,
		},
		{
			name:     "non-string value",
			key:      "line",
			pattern:  "*",
			payload:  map[string]any{"line": 42},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := Glob(tt.key, tt.pattern)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("Glob(%q, %q).Match() = %v, want %v", tt.key, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestPrefixFilter(t *testing.T) {
	filter := Prefix("file", "src/")

	match := newTestRecord(1, map[string]any{"file": "src/main.go"})
	if !filter.Match(match) {
		t.Error("Prefix should match 'src/main.go'")
	}

	noMatch := newTestRecord(2, map[string]any{"file": "test/main.go"})
	if filter.Match(noMatch) {
		t.Error("Prefix should not match 'test/main.go'")
	}
}

func TestSuffixFilter(t *testing.T) {
	filter := Suffix("file", "_test.go")

	match := newTestRecord(1, map[string]any{"file": "main_test.go"})
	if !filter.Match(match) {
		t.Error("Suffix should match 'main_test.go'")
	}

	noMatch := newTestRecord(2, map[string]any{"file": "main.go"})
	if filter.Match(noMatch) {
		t.Error("Suffix should not match 'main.go'")
	}
}

func TestContainsFilter(t *testing.T) {
	filter := Contains("file", "internal")

	match := newTestRecord(1, map[string]any{"file": "pkg/internal/util.go"})
	if !filter.Match(match) {
		t.Error("Contains should match path with 'internal'")
	}

	noMatch := newTestRecord(2, map[string]any{"file": "pkg/public/util.go"})
	if filter.Match(noMatch) {
		t.Error("Contains should not match path without 'internal'")
	}
}

func TestExistsFilter(t *testing.T) {
	filter := Exists("metadata")

	exists := newTestRecord(1, map[string]any{"metadata": "value"})
	if !filter.Match(exists) {
		t.Error("Exists should match when key exists")
	}

	notExists := newTestRecord(2, map[string]any{"other": "value"})
	if filter.Match(notExists) {
		t.Error("Exists should not match when key is missing")
	}
}

func TestAndFilter(t *testing.T) {
	filter := And(
		Equal("lang", "go"),
		Equal("type", "function"),
	)

	match := newTestRecord(1, map[string]any{"lang": "go", "type": "function"})
	if !filter.Match(match) {
		t.Error("And should match when all conditions are true")
	}

	partial := newTestRecord(2, map[string]any{"lang": "go", "type": "class"})
	if filter.Match(partial) {
		t.Error("And should not match when some conditions are false")
	}
}

func TestOrFilter(t *testing.T) {
	filter := Or(
		Equal("lang", "go"),
		Equal("lang", "rust"),
	)

	goRecord := newTestRecord(1, map[string]any{"lang": "go"})
	if !filter.Match(goRecord) {
		t.Error("Or should match 'go'")
	}

	rustRecord := newTestRecord(2, map[string]any{"lang": "rust"})
	if !filter.Match(rustRecord) {
		t.Error("Or should match 'rust'")
	}

	pythonRecord := newTestRecord(3, map[string]any{"lang": "python"})
	if filter.Match(pythonRecord) {
		t.Error("Or should not match 'python'")
	}
}

func TestNotFilter(t *testing.T) {
	filter := Not(Equal("lang", "python"))

	goRecord := newTestRecord(1, map[string]any{"lang": "go"})
	if !filter.Match(goRecord) {
		t.Error("Not should match when inner filter doesn't match")
	}

	pythonRecord := newTestRecord(2, map[string]any{"lang": "python"})
	if filter.Match(pythonRecord) {
		t.Error("Not should not match when inner filter matches")
	}
}

func TestFilterFunc(t *testing.T) {
	// Custom filter that checks if line > 100
	filter := FilterFunc(func(r *Record) bool {
		if r.Payload == nil {
			return false
		}
		line, ok := r.Payload["line"].(int)
		return ok && line > 100
	})

	high := newTestRecord(1, map[string]any{"line": 150})
	if !filter.Match(high) {
		t.Error("Custom filter should match line > 100")
	}

	low := newTestRecord(2, map[string]any{"line": 50})
	if filter.Match(low) {
		t.Error("Custom filter should not match line <= 100")
	}
}

func TestComplexFilterCombinations(t *testing.T) {
	// Find Go files that are either functions or methods, but not tests
	filter := And(
		Equal("lang", "go"),
		Or(
			Equal("type", "function"),
			Equal("type", "method"),
		),
		Not(Suffix("file", "_test.go")),
	)

	goFunc := newTestRecord(1, map[string]any{
		"lang": "go",
		"type": "function",
		"file": "main.go",
	})
	if !filter.Match(goFunc) {
		t.Error("Should match Go function in non-test file")
	}

	goTestFunc := newTestRecord(2, map[string]any{
		"lang": "go",
		"type": "function",
		"file": "main_test.go",
	})
	if filter.Match(goTestFunc) {
		t.Error("Should not match Go function in test file")
	}

	goClass := newTestRecord(3, map[string]any{
		"lang": "go",
		"type": "struct",
		"file": "main.go",
	})
	if filter.Match(goClass) {
		t.Error("Should not match Go struct")
	}
}

func TestNumericComparisons(t *testing.T) {
	// Test int/int64/float comparisons
	record := newTestRecord(1, map[string]any{"count": 42})

	if !Equal("count", 42).Match(record) {
		t.Error("Should match int 42")
	}

	if !Equal("count", int64(42)).Match(record) {
		t.Error("Should match int64 42")
	}

	if !Equal("count", float64(42)).Match(record) {
		t.Error("Should match float64 42")
	}
}

func TestGreaterThanFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    float64
		payload  map[string]any
		expected bool
	}{
		{
			name:     "int greater than",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 75},
			expected: true,
		},
		{
			name:     "int equal (not greater)",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 50},
			expected: false,
		},
		{
			name:     "int less than",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 25},
			expected: false,
		},
		{
			name:     "float64 greater than",
			key:      "score",
			value:    0.5,
			payload:  map[string]any{"score": 0.75},
			expected: true,
		},
		{
			name:     "missing key",
			key:      "missing",
			value:    50,
			payload:  map[string]any{"other": 100},
			expected: false,
		},
		{
			name:     "non-numeric value",
			key:      "name",
			value:    50,
			payload:  map[string]any{"name": "test"},
			expected: false,
		},
		{
			name:     "nil payload",
			key:      "score",
			value:    50,
			payload:  nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := GreaterThan(tt.key, tt.value)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("GreaterThan(%q, %v).Match() = %v, want %v", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestGTAlias(t *testing.T) {
	record := newTestRecord(1, map[string]any{"score": 75})
	if !GT("score", 50).Match(record) {
		t.Error("GT should be an alias for GreaterThan")
	}
}

func TestGreaterThanOrEqualFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    float64
		payload  map[string]any
		expected bool
	}{
		{
			name:     "greater than",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 75},
			expected: true,
		},
		{
			name:     "equal",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 50},
			expected: true,
		},
		{
			name:     "less than",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 25},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := GreaterThanOrEqual(tt.key, tt.value)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("GreaterThanOrEqual(%q, %v).Match() = %v, want %v", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestGTEAlias(t *testing.T) {
	record := newTestRecord(1, map[string]any{"score": 50})
	if !GTE("score", 50).Match(record) {
		t.Error("GTE should be an alias for GreaterThanOrEqual")
	}
}

func TestLessThanFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    float64
		payload  map[string]any
		expected bool
	}{
		{
			name:     "int less than",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 25},
			expected: true,
		},
		{
			name:     "int equal (not less)",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 50},
			expected: false,
		},
		{
			name:     "int greater than",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 75},
			expected: false,
		},
		{
			name:     "float64 less than",
			key:      "score",
			value:    0.5,
			payload:  map[string]any{"score": 0.25},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := LessThan(tt.key, tt.value)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("LessThan(%q, %v).Match() = %v, want %v", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestLTAlias(t *testing.T) {
	record := newTestRecord(1, map[string]any{"score": 25})
	if !LT("score", 50).Match(record) {
		t.Error("LT should be an alias for LessThan")
	}
}

func TestLessThanOrEqualFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    float64
		payload  map[string]any
		expected bool
	}{
		{
			name:     "less than",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 25},
			expected: true,
		},
		{
			name:     "equal",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 50},
			expected: true,
		},
		{
			name:     "greater than",
			key:      "score",
			value:    50,
			payload:  map[string]any{"score": 75},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := LessThanOrEqual(tt.key, tt.value)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("LessThanOrEqual(%q, %v).Match() = %v, want %v", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestLTEAlias(t *testing.T) {
	record := newTestRecord(1, map[string]any{"score": 50})
	if !LTE("score", 50).Match(record) {
		t.Error("LTE should be an alias for LessThanOrEqual")
	}
}

func TestBetweenFilter(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		min      float64
		max      float64
		payload  map[string]any
		expected bool
	}{
		{
			name:     "in range",
			key:      "line",
			min:      10,
			max:      50,
			payload:  map[string]any{"line": 25},
			expected: true,
		},
		{
			name:     "at min boundary",
			key:      "line",
			min:      10,
			max:      50,
			payload:  map[string]any{"line": 10},
			expected: true,
		},
		{
			name:     "at max boundary",
			key:      "line",
			min:      10,
			max:      50,
			payload:  map[string]any{"line": 50},
			expected: true,
		},
		{
			name:     "below min",
			key:      "line",
			min:      10,
			max:      50,
			payload:  map[string]any{"line": 5},
			expected: false,
		},
		{
			name:     "above max",
			key:      "line",
			min:      10,
			max:      50,
			payload:  map[string]any{"line": 100},
			expected: false,
		},
		{
			name:     "float range",
			key:      "score",
			min:      0.3,
			max:      0.7,
			payload:  map[string]any{"score": 0.5},
			expected: true,
		},
		{
			name:     "missing key",
			key:      "missing",
			min:      10,
			max:      50,
			payload:  map[string]any{"other": 25},
			expected: false,
		},
		{
			name:     "non-numeric value",
			key:      "name",
			min:      10,
			max:      50,
			payload:  map[string]any{"name": "test"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := Between(tt.key, tt.min, tt.max)
			record := newTestRecord(1, tt.payload)
			result := filter.Match(record)
			if result != tt.expected {
				t.Errorf("Between(%q, %v, %v).Match() = %v, want %v", tt.key, tt.min, tt.max, result, tt.expected)
			}
		})
	}
}

func TestRangeFiltersWithDifferentNumericTypes(t *testing.T) {
	// Test that range filters work with various numeric types
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{"int", map[string]any{"val": 50}},
		{"int8", map[string]any{"val": int8(50)}},
		{"int16", map[string]any{"val": int16(50)}},
		{"int32", map[string]any{"val": int32(50)}},
		{"int64", map[string]any{"val": int64(50)}},
		{"uint", map[string]any{"val": uint(50)}},
		{"uint8", map[string]any{"val": uint8(50)}},
		{"uint16", map[string]any{"val": uint16(50)}},
		{"uint32", map[string]any{"val": uint32(50)}},
		{"uint64", map[string]any{"val": uint64(50)}},
		{"float32", map[string]any{"val": float32(50)}},
		{"float64", map[string]any{"val": float64(50)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := newTestRecord(1, tt.payload)

			if !GT("val", 25).Match(record) {
				t.Errorf("GT should match %s value 50 > 25", tt.name)
			}
			if !GTE("val", 50).Match(record) {
				t.Errorf("GTE should match %s value 50 >= 50", tt.name)
			}
			if !LT("val", 75).Match(record) {
				t.Errorf("LT should match %s value 50 < 75", tt.name)
			}
			if !LTE("val", 50).Match(record) {
				t.Errorf("LTE should match %s value 50 <= 50", tt.name)
			}
			if !Between("val", 25, 75).Match(record) {
				t.Errorf("Between should match %s value 50 in [25, 75]", tt.name)
			}
		})
	}
}

func TestRangeFiltersInCombinations(t *testing.T) {
	// Test combining range filters with other filters
	record := newTestRecord(1, map[string]any{
		"file":  "main.go",
		"line":  150,
		"score": 0.8,
	})

	// Go files with line > 100 and score >= 0.5
	filter := And(
		Suffix("file", ".go"),
		GT("line", 100),
		GTE("score", 0.5),
	)

	if !filter.Match(record) {
		t.Error("Combined filter should match")
	}

	// Test with Or
	orFilter := Or(
		LT("line", 50),
		GT("line", 100),
	)

	if !orFilter.Match(record) {
		t.Error("Or filter should match line > 100")
	}

	// Test with Not
	notFilter := Not(Between("score", 0.0, 0.5))
	if !notFilter.Match(record) {
		t.Error("Not(Between) should match score 0.8")
	}
}
