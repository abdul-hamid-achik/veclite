package veclite

import (
	"path/filepath"
	"strings"
)

// Filter is an interface for filtering records based on payload values.
type Filter interface {
	// Match returns true if the record matches the filter criteria.
	Match(r *Record) bool
}

// FilterFunc is a function adapter for the Filter interface.
type FilterFunc func(r *Record) bool

// Match implements Filter interface.
func (f FilterFunc) Match(r *Record) bool {
	return f(r)
}

// equalFilter matches records where payload[key] equals value.
type equalFilter struct {
	key   string
	value any
}

func (f *equalFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	return compareValues(v, f.value)
}

// Equal creates a filter that matches records where payload[key] equals value.
func Equal(key string, value any) Filter {
	return &equalFilter{key: key, value: value}
}

// notEqualFilter matches records where payload[key] does not equal value.
type notEqualFilter struct {
	key   string
	value any
}

func (f *notEqualFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return true
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return true
	}
	return !compareValues(v, f.value)
}

// NotEqual creates a filter that matches records where payload[key] does not equal value.
func NotEqual(key string, value any) Filter {
	return &notEqualFilter{key: key, value: value}
}

// inFilter matches records where payload[key] is in the given values.
type inFilter struct {
	key    string
	values []any
}

func (f *inFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	for _, val := range f.values {
		if compareValues(v, val) {
			return true
		}
	}
	return false
}

// In creates a filter that matches records where payload[key] is in the given values.
func In(key string, values ...any) Filter {
	return &inFilter{key: key, values: values}
}

// notInFilter matches records where payload[key] is not in the given values.
type notInFilter struct {
	key    string
	values []any
}

func (f *notInFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return true
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return true
	}
	for _, val := range f.values {
		if compareValues(v, val) {
			return false
		}
	}
	return true
}

// NotIn creates a filter that matches records where payload[key] is not in the given values.
func NotIn(key string, values ...any) Filter {
	return &notInFilter{key: key, values: values}
}

// globFilter matches records where payload[key] matches a glob pattern.
type globFilter struct {
	key     string
	pattern string
}

func (f *globFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	str, ok := v.(string)
	if !ok {
		return false
	}
	matched, _ := filepath.Match(f.pattern, str)
	return matched
}

// Glob creates a filter that matches records where payload[key] matches the glob pattern.
func Glob(key, pattern string) Filter {
	return &globFilter{key: key, pattern: pattern}
}

// prefixFilter matches records where payload[key] has the given prefix.
type prefixFilter struct {
	key    string
	prefix string
}

func (f *prefixFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	str, ok := v.(string)
	if !ok {
		return false
	}
	return strings.HasPrefix(str, f.prefix)
}

// Prefix creates a filter that matches records where payload[key] has the given prefix.
func Prefix(key, prefix string) Filter {
	return &prefixFilter{key: key, prefix: prefix}
}

// suffixFilter matches records where payload[key] has the given suffix.
type suffixFilter struct {
	key    string
	suffix string
}

func (f *suffixFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	str, ok := v.(string)
	if !ok {
		return false
	}
	return strings.HasSuffix(str, f.suffix)
}

// Suffix creates a filter that matches records where payload[key] has the given suffix.
func Suffix(key, suffix string) Filter {
	return &suffixFilter{key: key, suffix: suffix}
}

// containsFilter matches records where payload[key] contains the substring.
type containsFilter struct {
	key    string
	substr string
}

func (f *containsFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	str, ok := v.(string)
	if !ok {
		return false
	}
	return strings.Contains(str, f.substr)
}

// Contains creates a filter that matches records where payload[key] contains the substring.
func Contains(key, substr string) Filter {
	return &containsFilter{key: key, substr: substr}
}

// existsFilter matches records where payload[key] exists.
type existsFilter struct {
	key string
}

func (f *existsFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	_, ok := r.Payload[f.key]
	return ok
}

// Exists creates a filter that matches records where payload[key] exists.
func Exists(key string) Filter {
	return &existsFilter{key: key}
}

// andFilter matches records that match all filters.
type andFilter struct {
	filters []Filter
}

func (f *andFilter) Match(r *Record) bool {
	for _, filter := range f.filters {
		if !filter.Match(r) {
			return false
		}
	}
	return true
}

// And creates a filter that matches records matching all given filters.
func And(filters ...Filter) Filter {
	return &andFilter{filters: filters}
}

// orFilter matches records that match any filter.
type orFilter struct {
	filters []Filter
}

func (f *orFilter) Match(r *Record) bool {
	for _, filter := range f.filters {
		if filter.Match(r) {
			return true
		}
	}
	return false
}

// Or creates a filter that matches records matching any given filter.
func Or(filters ...Filter) Filter {
	return &orFilter{filters: filters}
}

// notFilter negates a filter.
type notFilter struct {
	filter Filter
}

func (f *notFilter) Match(r *Record) bool {
	return !f.filter.Match(r)
}

// Not creates a filter that negates the given filter.
func Not(filter Filter) Filter {
	return &notFilter{filter: filter}
}

// greaterThanFilter matches records where payload[key] > value.
type greaterThanFilter struct {
	key   string
	value float64
}

func (f *greaterThanFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	num, ok := toFloat64(v)
	if !ok {
		return false
	}
	return num > f.value
}

// GreaterThan creates a filter that matches records where payload[key] > value.
func GreaterThan(key string, value float64) Filter {
	return &greaterThanFilter{key: key, value: value}
}

// GT is an alias for GreaterThan.
func GT(key string, value float64) Filter {
	return GreaterThan(key, value)
}

// greaterThanOrEqualFilter matches records where payload[key] >= value.
type greaterThanOrEqualFilter struct {
	key   string
	value float64
}

func (f *greaterThanOrEqualFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	num, ok := toFloat64(v)
	if !ok {
		return false
	}
	return num >= f.value
}

// GreaterThanOrEqual creates a filter that matches records where payload[key] >= value.
func GreaterThanOrEqual(key string, value float64) Filter {
	return &greaterThanOrEqualFilter{key: key, value: value}
}

// GTE is an alias for GreaterThanOrEqual.
func GTE(key string, value float64) Filter {
	return GreaterThanOrEqual(key, value)
}

// lessThanFilter matches records where payload[key] < value.
type lessThanFilter struct {
	key   string
	value float64
}

func (f *lessThanFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	num, ok := toFloat64(v)
	if !ok {
		return false
	}
	return num < f.value
}

// LessThan creates a filter that matches records where payload[key] < value.
func LessThan(key string, value float64) Filter {
	return &lessThanFilter{key: key, value: value}
}

// LT is an alias for LessThan.
func LT(key string, value float64) Filter {
	return LessThan(key, value)
}

// lessThanOrEqualFilter matches records where payload[key] <= value.
type lessThanOrEqualFilter struct {
	key   string
	value float64
}

func (f *lessThanOrEqualFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	num, ok := toFloat64(v)
	if !ok {
		return false
	}
	return num <= f.value
}

// LessThanOrEqual creates a filter that matches records where payload[key] <= value.
func LessThanOrEqual(key string, value float64) Filter {
	return &lessThanOrEqualFilter{key: key, value: value}
}

// LTE is an alias for LessThanOrEqual.
func LTE(key string, value float64) Filter {
	return LessThanOrEqual(key, value)
}

// betweenFilter matches records where min <= payload[key] <= max.
type betweenFilter struct {
	key string
	min float64
	max float64
}

func (f *betweenFilter) Match(r *Record) bool {
	if r.Payload == nil {
		return false
	}
	v, ok := r.Payload[f.key]
	if !ok {
		return false
	}
	num, ok := toFloat64(v)
	if !ok {
		return false
	}
	return num >= f.min && num <= f.max
}

// Between creates a filter that matches records where min <= payload[key] <= max.
func Between(key string, min, max float64) Filter {
	return &betweenFilter{key: key, min: min, max: max}
}

// toFloat64 converts a numeric value to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// compareValues compares two values for equality.
func compareValues(a, b any) bool {
	// Handle nil
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Try direct comparison for common types
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case int:
		return compareInts(av, b)
	case int64:
		return compareInt64s(av, b)
	case float64:
		return compareFloat64s(av, b)
	case float32:
		return compareFloat32s(av, b)
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	}

	// Fallback: compare as strings
	return false
}

func compareInts(a int, b any) bool {
	switch bv := b.(type) {
	case int:
		return a == bv
	case int64:
		return int64(a) == bv
	case float64:
		return float64(a) == bv
	}
	return false
}

func compareInt64s(a int64, b any) bool {
	switch bv := b.(type) {
	case int:
		return a == int64(bv)
	case int64:
		return a == bv
	case float64:
		return float64(a) == bv
	}
	return false
}

func compareFloat64s(a float64, b any) bool {
	switch bv := b.(type) {
	case int:
		return a == float64(bv)
	case int64:
		return a == float64(bv)
	case float64:
		return a == bv
	case float32:
		return float32(a) == bv
	}
	return false
}

func compareFloat32s(a float32, b any) bool {
	switch bv := b.(type) {
	case int:
		return a == float32(bv)
	case int64:
		return a == float32(bv)
	case float64:
		return a == float32(bv)
	case float32:
		return a == bv
	}
	return false
}
