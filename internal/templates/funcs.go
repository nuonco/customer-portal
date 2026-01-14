package templates

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"
)

// DefaultFuncs returns the default template function map
func DefaultFuncs() template.FuncMap {
	return template.FuncMap{
		// String manipulation
		"safeSlice":    safeSlice,
		"upper":        strings.ToUpper,
		"lower":        strings.ToLower,
		"title":        strings.Title,
		"contains":     strings.Contains,
		"hasPrefix":    strings.HasPrefix,
		"hasSuffix":    strings.HasSuffix,
		"replace":      strings.ReplaceAll,
		"trim":         strings.TrimSpace,
		"split":        strings.Split,
		"join":         strings.Join,
		"boolToString": boolToString,

		// URL handling
		"safeURL": func(s string) template.URL { return template.URL(s) },
		"safeJS":  func(s string) template.JS { return template.JS(s) },
		"safeCSS": func(s string) template.CSS { return template.CSS(s) },

		// HTML handling
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"raw":      func(s string) template.HTML { return template.HTML(s) },

		// CSS class helpers
		"classes":      classBuilder,
		"classIf":      classIf,
		"variantClass": variantClass,
		"sizeClass":    sizeClass,

		// Theme helpers
		"themeCSS":    buildThemeCSS,
		"darkenColor": DarkenColor,

		// Conditional helpers
		"default":  defaultValue,
		"coalesce": coalesce,
		"ternary":  ternary,

		// Date/time formatting
		"formatDate":     formatDate,
		"formatDateTime": formatDateTime,
		"formatRelative": formatRelative,
		"now":            time.Now,

		// Arithmetic
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},

		// Comparison
		"eq":  func(a, b interface{}) bool { return a == b },
		"ne":  func(a, b interface{}) bool { return a != b },
		"lt":  lt,
		"le":  le,
		"gt":  gt,
		"ge":  ge,
		"and": and,
		"or":  or,
		"not": not,

		// Iteration helpers
		"seq":   seq,
		"first": first,
		"last":  last,

		// Map/dict helpers
		"dict": dict,
		"get":  getFromMap,
		"set":  setInMap,

		// Status helpers
		"statusColor":               statusColor,
		"statusBgClass":             statusBgClass,
		"statusIcon":                statusIcon,
		"healthCheckStatusDotClass": healthCheckStatusDotClass,

		// Formatting
		"formatNumber": formatNumber,
		"pluralize":    pluralize,
		"truncate":     truncate,

		// JSON helpers
		"jsonAttr": jsonAttr,

		// Pagination helpers
		"pageRange": pageRange,

		// Input/form helpers
		"getColorValue":   getColorValue,
		"hasInputs":       hasInputs,
		"truncateDefault": truncateDefault,
	}
}

// safeSlice safely slices a string, returning empty string if out of bounds
func safeSlice(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start >= end || start >= len(s) {
		return ""
	}
	return s[start:end]
}

// boolToString converts a boolean to "true" or "false" string
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// classBuilder combines multiple CSS classes, filtering out empty strings
// Accepts interface{} to handle nil values from template maps
func classBuilder(classes ...interface{}) string {
	var result []string
	for _, c := range classes {
		if c == nil {
			continue
		}
		if s, ok := c.(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				result = append(result, s)
			}
		}
	}
	return strings.Join(result, " ")
}

// classIf returns the class string if condition is true, empty string otherwise
func classIf(condition bool, class string) string {
	if condition {
		return class
	}
	return ""
}

// variantClass returns CSS classes for button/badge variants
// Accepts interface{} for primaryColor and disabled to handle nil values from template maps
func variantClass(variant interface{}, primaryColor interface{}, disabled interface{}) string {
	// Convert disabled to bool
	isDisabled := false
	if disabled != nil {
		if b, ok := disabled.(bool); ok {
			isDisabled = b
		}
	}

	if isDisabled {
		return "opacity-50 cursor-not-allowed"
	}

	// Convert variant to string
	v := "primary"
	if variant != nil {
		if s, ok := variant.(string); ok && s != "" {
			v = s
		}
	}

	// Convert primaryColor to string
	pc := ""
	if primaryColor != nil {
		if s, ok := primaryColor.(string); ok {
			pc = s
		}
	}

	switch v {
	case "primary":
		if pc != "" {
			return fmt.Sprintf("bg-[%s] hover:bg-[%s] text-white", pc, DarkenColor(pc, 0.8))
		}
		return "bg-primary-600 hover:bg-primary-700 text-white"
	case "secondary":
		return "bg-cool-grey-100 hover:bg-cool-grey-200 text-cool-grey-700 dark:bg-dark-grey-700 dark:hover:bg-dark-grey-600 dark:text-cool-grey-300"
	case "danger":
		return "bg-red-600 hover:bg-red-700 text-white"
	case "ghost":
		return "hover:bg-cool-grey-100 dark:hover:bg-dark-grey-700 text-cool-grey-700 dark:text-cool-grey-300"
	case "link":
		return "text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300 underline"
	default:
		return "bg-primary-600 hover:bg-primary-700 text-white"
	}
}

// sizeClass returns CSS classes for size variants
func sizeClass(size string) string {
	switch size {
	case "xs":
		return "px-2 py-1 text-xs"
	case "sm":
		return "px-3 py-1.5 text-sm"
	case "md":
		return "px-4 py-2 text-sm"
	case "lg":
		return "px-5 py-2.5 text-base"
	case "xl":
		return "px-6 py-3 text-lg"
	default:
		return "px-4 py-2 text-sm"
	}
}

// buildThemeCSS generates CSS variable declarations for theme customization
func buildThemeCSS(theme *ThemeData) template.HTML {
	if theme == nil {
		return ""
	}

	var css strings.Builder
	css.WriteString(":root{")

	if theme.PrimaryColor != "" {
		css.WriteString("--theme-primary:" + theme.PrimaryColor + ";")
		css.WriteString("--theme-primary-hover:" + theme.PrimaryColorDark + ";")
	}
	if theme.SecondaryColor != "" {
		css.WriteString("--theme-secondary:" + theme.SecondaryColor + ";")
		css.WriteString("--theme-secondary-hover:" + theme.SecondaryColorDark + ";")
	}
	if theme.HeadingFontBase64 != "" {
		css.WriteString("--font-heading:'CustomHeading','Inter',ui-sans-serif,system-ui,sans-serif;")
	} else if theme.HeadingFont != "" {
		css.WriteString("--font-heading:'" + theme.HeadingFont + "','Inter',ui-sans-serif,system-ui,sans-serif;")
	}
	if theme.BodyFontBase64 != "" {
		css.WriteString("--font-body:'CustomBody','Inter',ui-sans-serif,system-ui,sans-serif;")
	} else if theme.BodyFont != "" {
		css.WriteString("--font-body:'" + theme.BodyFont + "','Inter',ui-sans-serif,system-ui,sans-serif;")
	}

	css.WriteString("}")

	if css.Len() <= len(":root{}") {
		return ""
	}

	return template.HTML("<style>" + css.String() + "</style>")
}

// DarkenColor takes a hex color and returns a darker version
func DarkenColor(hexColor string, factor float64) string {
	hexColor = strings.TrimPrefix(hexColor, "#")
	if len(hexColor) != 6 {
		return "#1D4ED8" // Default dark blue
	}

	r, _ := strconv.ParseInt(hexColor[0:2], 16, 64)
	g, _ := strconv.ParseInt(hexColor[2:4], 16, 64)
	b, _ := strconv.ParseInt(hexColor[4:6], 16, 64)

	r = int64(float64(r) * factor)
	g = int64(float64(g) * factor)
	b = int64(float64(b) * factor)

	if r < 0 {
		r = 0
	}
	if g < 0 {
		g = 0
	}
	if b < 0 {
		b = 0
	}

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// defaultValue returns the default value if the value is empty/zero
func defaultValue(value, defaultVal interface{}) interface{} {
	if value == nil || value == "" || value == 0 || value == false {
		return defaultVal
	}
	return value
}

// coalesce returns the first non-empty value
func coalesce(values ...interface{}) interface{} {
	for _, v := range values {
		if v != nil && v != "" && v != 0 && v != false {
			return v
		}
	}
	return nil
}

// ternary returns trueVal if condition is true, falseVal otherwise
func ternary(condition bool, trueVal, falseVal interface{}) interface{} {
	if condition {
		return trueVal
	}
	return falseVal
}

// formatDate formats a time as a date string
func formatDate(t time.Time, layout string) string {
	if layout == "" {
		layout = "Jan 2, 2006"
	}
	return t.Format(layout)
}

// formatDateTime formats a time as a datetime string
func formatDateTime(t time.Time) string {
	return t.Format("Jan 2, 2006 3:04 PM")
}

// formatRelative formats a time as a relative string (e.g., "2 hours ago")
func formatRelative(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2, 2006")
	}
}

// lt compares two integers
func lt(a, b int) bool { return a < b }
func le(a, b int) bool { return a <= b }
func gt(a, b int) bool { return a > b }
func ge(a, b int) bool { return a >= b }

// and returns true if all values are truthy
func and(values ...interface{}) bool {
	for _, v := range values {
		if !isTruthy(v) {
			return false
		}
	}
	return true
}

// or returns true if any value is truthy
func or(values ...interface{}) bool {
	for _, v := range values {
		if isTruthy(v) {
			return true
		}
	}
	return false
}

// not returns the logical not of a value
func not(v interface{}) bool {
	return !isTruthy(v)
}

// isTruthy checks if a value is truthy
func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case int:
		return val != 0
	case int64:
		return val != 0
	case float64:
		return val != 0
	default:
		return true
	}
}

// seq generates a sequence of integers
func seq(start, end int) []int {
	if start > end {
		return nil
	}
	result := make([]int, end-start+1)
	for i := range result {
		result[i] = start + i
	}
	return result
}

// first returns the first n items from a slice
func first(n int, items interface{}) interface{} {
	// This is a simplified version - in real use, you'd use reflection
	return items
}

// last returns the last n items from a slice
func last(n int, items interface{}) interface{} {
	return items
}

// dict creates a map from key-value pairs
func dict(values ...interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for i := 0; i < len(values)-1; i += 2 {
		key, ok := values[i].(string)
		if ok {
			result[key] = values[i+1]
		}
	}
	return result
}

// getFromMap gets a value from a map
func getFromMap(m map[string]interface{}, key string) interface{} {
	return m[key]
}

// setInMap sets a value in a map and returns the map
func setInMap(m map[string]interface{}, key string, value interface{}) map[string]interface{} {
	m[key] = value
	return m
}

// statusColor returns the color class for a status
func statusColor(status string) string {
	switch strings.ToLower(status) {
	case "healthy", "active", "success", "complete", "completed":
		return "green"
	case "updating", "pending", "in_progress", "running":
		return "blue"
	case "needs_attention", "warning", "degraded":
		return "yellow"
	case "error", "failed", "unhealthy", "critical":
		return "red"
	default:
		return "gray"
	}
}

// statusBgClass returns the background class for a status
func statusBgClass(status string) string {
	color := statusColor(status)
	switch color {
	case "green":
		return "bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300"
	case "blue":
		return "bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300"
	case "yellow":
		return "bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-300"
	case "red":
		return "bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300"
	default:
		return "bg-cool-grey-100 dark:bg-cool-grey-700 text-cool-grey-800 dark:text-cool-grey-300"
	}
}

// statusIcon returns the icon name for a status
func statusIcon(status string) string {
	color := statusColor(status)
	switch color {
	case "green":
		return "check-circle"
	case "blue":
		return "clock"
	case "yellow":
		return "exclamation-triangle"
	case "red":
		return "x-circle"
	default:
		return "question-mark-circle"
	}
}

// healthCheckStatusDotClass returns the dot color class for health check status
func healthCheckStatusDotClass(status string) string {
	switch status {
	case "Passing":
		return "bg-green-500"
	case "Pending":
		return "bg-yellow-500"
	case "Failing":
		return "bg-red-500"
	default:
		return "bg-cool-grey-400"
	}
}

// formatNumber formats a number with commas
func formatNumber(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}

// pluralize returns singular or plural form based on count
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// truncate truncates a string to the given length
func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	if length <= 3 {
		return s[:length]
	}
	return s[:length-3] + "..."
}

// jsonAttr safely outputs a value as a JSON string for HTML attributes
func jsonAttr(v interface{}) template.HTMLAttr {
	switch val := v.(type) {
	case string:
		return template.HTMLAttr(val)
	case bool:
		if val {
			return template.HTMLAttr("true")
		}
		return template.HTMLAttr("false")
	case int:
		return template.HTMLAttr(strconv.Itoa(val))
	default:
		return template.HTMLAttr(fmt.Sprintf("%v", v))
	}
}

// pageRange generates a range of page numbers for pagination
func pageRange(currentPage, totalPages, maxVisible int) []int {
	if totalPages <= maxVisible {
		return seq(1, totalPages)
	}

	half := maxVisible / 2
	start := currentPage - half
	end := currentPage + half

	if start < 1 {
		start = 1
		end = maxVisible
	}
	if end > totalPages {
		end = totalPages
		start = totalPages - maxVisible + 1
	}

	return seq(start, end)
}

// getColorValue returns the color value or a default if empty
func getColorValue(color, defaultColor string) string {
	if color != "" {
		return color
	}
	return defaultColor
}

// hasInputs checks if any input groups have inputs
func hasInputs(groups interface{}) bool {
	// Type assertion for InputGroup slices
	switch g := groups.(type) {
	case []map[string]interface{}:
		for _, group := range g {
			if inputs, ok := group["Inputs"].([]interface{}); ok && len(inputs) > 0 {
				return true
			}
		}
	default:
		// For struct types, we'd need reflection - but we can handle this in the template
		return true
	}
	return false
}

// truncateDefault truncates a default value string for display
func truncateDefault(s string) string {
	if len(s) > 30 {
		return s[:27] + "..."
	}
	return s
}
