package overrides

import (
	"bytes"
	"context"
	"html/template"
	"reflect"

	"github.com/a-h/templ"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/components"
)

// renderTemplComponent renders a templ.Component to template.HTML.
func renderTemplComponent(c templ.Component) template.HTML {
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		return template.HTML("<!-- component error: " + err.Error() + " -->")
	}
	return template.HTML(buf.String())
}

// RenderStatusBadge renders a status badge component.
// Usage in templates:
//
//	{{ statusBadge .Status }}
//	{{ statusBadge .Status "install" }}
//	{{ statusBadge .Status "install" "md" }}
func RenderStatusBadge(status string, args ...string) template.HTML {
	props := components.StatusBadgeProps{Status: status}
	if len(args) > 0 {
		props.Variant = args[0]
	}
	if len(args) > 1 {
		props.Size = args[1]
	}
	return renderTemplComponent(components.StatusBadge(props))
}

// RenderAlert renders an alert banner component.
// Usage in templates:
//
//	{{ alert "info" "Your message here" }}
//	{{ alert "success" "Changes saved!" }}
//	{{ alert "error" .ErrorMessage }}
//	{{ alert "warning" "Please review" }}
func RenderAlert(alertType, message string) template.HTML {
	props := components.AlertProps{
		Type:    alertType,
		Message: message,
	}
	return renderTemplComponent(components.Alert(props))
}

// RenderEmptyState renders an empty state component.
// Usage in templates:
//
//	{{ emptyState "No items" "Try creating one" }}
//	{{ emptyState "All done!" "No pending tasks" "check-circle" }}
//	{{ emptyState "Loading..." "Please wait" "refresh" }}
func RenderEmptyState(title, message string, args ...string) template.HTML {
	props := components.EmptyStateProps{
		Title:   title,
		Message: message,
	}
	if len(args) > 0 {
		props.Icon = args[0] // "inbox" | "check-circle" | "refresh"
	}
	return renderTemplComponent(components.EmptyState(props))
}

// PaginationData represents pagination information that can be passed to the pagination component.
// This can be extracted from InstallsPageData or passed directly.
type PaginationData struct {
	CurrentPage  int    `json:"current_page"`
	TotalPages   int    `json:"total_pages"`
	HasPrevious  bool   `json:"has_previous"`
	HasNext      bool   `json:"has_next"`
	PreviousPage int    `json:"previous_page"`
	NextPage     int    `json:"next_page"`
	BaseURL      string `json:"base_url"`
}

// RenderPagination renders a pagination component.
// Usage in templates:
//
//	{{ pagination .PageData }}  (when PageData has pagination fields)
//	{{ pagination (paginationData .CurrentPage .TotalPages .HasPrevious .HasNext .PreviousPage .NextPage "?page=") }}
func RenderPagination(data interface{}) template.HTML {
	props := extractPaginationProps(data)
	if props.TotalPages <= 1 {
		return template.HTML("") // Don't render pagination for single page
	}
	return renderTemplComponent(components.Pagination(props))
}

// extractPaginationProps extracts pagination props from various data types.
func extractPaginationProps(data interface{}) components.PaginationProps {
	if data == nil {
		return components.PaginationProps{}
	}

	// Handle PaginationData directly
	if pd, ok := data.(PaginationData); ok {
		return components.PaginationProps{
			CurrentPage:  pd.CurrentPage,
			TotalPages:   pd.TotalPages,
			HasPrevious:  pd.HasPrevious,
			HasNext:      pd.HasNext,
			PreviousPage: pd.PreviousPage,
			NextPage:     pd.NextPage,
			BaseURL:      pd.BaseURL,
		}
	}

	// Handle pointer to PaginationData
	if pd, ok := data.(*PaginationData); ok && pd != nil {
		return components.PaginationProps{
			CurrentPage:  pd.CurrentPage,
			TotalPages:   pd.TotalPages,
			HasPrevious:  pd.HasPrevious,
			HasNext:      pd.HasNext,
			PreviousPage: pd.PreviousPage,
			NextPage:     pd.NextPage,
			BaseURL:      pd.BaseURL,
		}
	}

	// Handle InstallsPageData directly
	if ipd, ok := data.(InstallsPageData); ok {
		return components.PaginationProps{
			CurrentPage:  ipd.CurrentPage,
			TotalPages:   ipd.TotalPages,
			HasPrevious:  ipd.HasPrevious,
			HasNext:      ipd.HasNext,
			PreviousPage: ipd.PreviousPage,
			NextPage:     ipd.NextPage,
			BaseURL:      "?page=",
			ShowingFrom:  ipd.ShowingFrom,
			ShowingTo:    ipd.ShowingTo,
			TotalCount:   int(ipd.TotalCount),
		}
	}

	// Handle pointer to InstallsPageData
	if ipd, ok := data.(*InstallsPageData); ok && ipd != nil {
		return components.PaginationProps{
			CurrentPage:  ipd.CurrentPage,
			TotalPages:   ipd.TotalPages,
			HasPrevious:  ipd.HasPrevious,
			HasNext:      ipd.HasNext,
			PreviousPage: ipd.PreviousPage,
			NextPage:     ipd.NextPage,
			BaseURL:      "?page=",
			ShowingFrom:  ipd.ShowingFrom,
			ShowingTo:    ipd.ShowingTo,
			TotalCount:   int(ipd.TotalCount),
		}
	}

	// Handle map (for flexibility with custom structures)
	if m, ok := data.(map[string]interface{}); ok {
		return extractPaginationFromMap(m)
	}

	// Try reflection for struct types with pagination fields
	return extractPaginationFromReflect(data)
}

// extractPaginationFromMap extracts pagination props from a map.
func extractPaginationFromMap(m map[string]interface{}) components.PaginationProps {
	props := components.PaginationProps{}

	if v, ok := m["CurrentPage"].(int); ok {
		props.CurrentPage = v
	} else if v, ok := m["current_page"].(int); ok {
		props.CurrentPage = v
	}

	if v, ok := m["TotalPages"].(int); ok {
		props.TotalPages = v
	} else if v, ok := m["total_pages"].(int); ok {
		props.TotalPages = v
	}

	if v, ok := m["HasPrevious"].(bool); ok {
		props.HasPrevious = v
	} else if v, ok := m["has_previous"].(bool); ok {
		props.HasPrevious = v
	}

	if v, ok := m["HasNext"].(bool); ok {
		props.HasNext = v
	} else if v, ok := m["has_next"].(bool); ok {
		props.HasNext = v
	}

	if v, ok := m["PreviousPage"].(int); ok {
		props.PreviousPage = v
	} else if v, ok := m["previous_page"].(int); ok {
		props.PreviousPage = v
	}

	if v, ok := m["NextPage"].(int); ok {
		props.NextPage = v
	} else if v, ok := m["next_page"].(int); ok {
		props.NextPage = v
	}

	if v, ok := m["BaseURL"].(string); ok {
		props.BaseURL = v
	} else if v, ok := m["base_url"].(string); ok {
		props.BaseURL = v
	} else {
		props.BaseURL = "?page="
	}

	if v, ok := m["ShowingFrom"].(int); ok {
		props.ShowingFrom = v
	} else if v, ok := m["showing_from"].(int); ok {
		props.ShowingFrom = v
	}

	if v, ok := m["ShowingTo"].(int); ok {
		props.ShowingTo = v
	} else if v, ok := m["showing_to"].(int); ok {
		props.ShowingTo = v
	}

	if v, ok := m["TotalCount"].(int); ok {
		props.TotalCount = v
	} else if v, ok := m["total_count"].(int); ok {
		props.TotalCount = v
	}

	return props
}

// extractPaginationFromReflect uses reflection to extract pagination fields.
func extractPaginationFromReflect(data interface{}) components.PaginationProps {
	props := components.PaginationProps{BaseURL: "?page="}

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return props
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return props
	}

	if f := v.FieldByName("CurrentPage"); f.IsValid() && f.Kind() == reflect.Int {
		props.CurrentPage = int(f.Int())
	}
	if f := v.FieldByName("TotalPages"); f.IsValid() && f.Kind() == reflect.Int {
		props.TotalPages = int(f.Int())
	}
	if f := v.FieldByName("HasPrevious"); f.IsValid() && f.Kind() == reflect.Bool {
		props.HasPrevious = f.Bool()
	}
	if f := v.FieldByName("HasNext"); f.IsValid() && f.Kind() == reflect.Bool {
		props.HasNext = f.Bool()
	}
	if f := v.FieldByName("PreviousPage"); f.IsValid() && f.Kind() == reflect.Int {
		props.PreviousPage = int(f.Int())
	}
	if f := v.FieldByName("NextPage"); f.IsValid() && f.Kind() == reflect.Int {
		props.NextPage = int(f.Int())
	}
	if f := v.FieldByName("BaseURL"); f.IsValid() && f.Kind() == reflect.String {
		props.BaseURL = f.String()
	}
	if f := v.FieldByName("ShowingFrom"); f.IsValid() && f.Kind() == reflect.Int {
		props.ShowingFrom = int(f.Int())
	}
	if f := v.FieldByName("ShowingTo"); f.IsValid() && f.Kind() == reflect.Int {
		props.ShowingTo = int(f.Int())
	}
	if f := v.FieldByName("TotalCount"); f.IsValid() {
		props.TotalCount = int(f.Int())
	}

	return props
}

// MakePaginationData is a helper function to create PaginationData in templates.
// Usage in templates:
//
//	{{ pagination (paginationData 1 10 false true 0 2 "?page=") }}
func MakePaginationData(currentPage, totalPages int, hasPrevious, hasNext bool, previousPage, nextPage int, baseURL string) PaginationData {
	return PaginationData{
		CurrentPage:  currentPage,
		TotalPages:   totalPages,
		HasPrevious:  hasPrevious,
		HasNext:      hasNext,
		PreviousPage: previousPage,
		NextPage:     nextPage,
		BaseURL:      baseURL,
	}
}
