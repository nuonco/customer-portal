package overrides

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestRenderStatusBadge(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		args     []string
		contains []string
	}{
		{
			name:     "active status",
			status:   "active",
			contains: []string{"bg-green-100", "active"},
		},
		{
			name:     "pending status",
			status:   "pending",
			contains: []string{"bg-blue-100", "pending"},
		},
		{
			name:     "failed status",
			status:   "failed",
			contains: []string{"bg-red-100", "failed"},
		},
		{
			name:     "needs_attention status",
			status:   "needs_attention",
			contains: []string{"bg-orange-100", "needs_attention"},
		},
		{
			name:     "with variant",
			status:   "active",
			args:     []string{"install"},
			contains: []string{"bg-green-100"},
		},
		{
			name:     "with size md",
			status:   "active",
			args:     []string{"install", "md"},
			contains: []string{"px-2.5", "py-1", "text-sm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := RenderStatusBadge(tt.status, tt.args...)
			htmlStr := string(html)

			for _, want := range tt.contains {
				if !strings.Contains(htmlStr, want) {
					t.Errorf("RenderStatusBadge() = %v, want to contain %v", htmlStr, want)
				}
			}
		})
	}
}

func TestRenderAlert(t *testing.T) {
	tests := []struct {
		name      string
		alertType string
		message   string
		contains  []string
	}{
		{
			name:      "info alert",
			alertType: "info",
			message:   "This is info",
			contains:  []string{"bg-blue-50", "This is info"},
		},
		{
			name:      "success alert",
			alertType: "success",
			message:   "Success!",
			contains:  []string{"bg-green-50", "Success!"},
		},
		{
			name:      "error alert",
			alertType: "error",
			message:   "Something went wrong",
			contains:  []string{"bg-red-50", "Something went wrong"},
		},
		{
			name:      "warning alert",
			alertType: "warning",
			message:   "Be careful",
			contains:  []string{"bg-orange-50", "Be careful"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := RenderAlert(tt.alertType, tt.message)
			htmlStr := string(html)

			for _, want := range tt.contains {
				if !strings.Contains(htmlStr, want) {
					t.Errorf("RenderAlert() = %v, want to contain %v", htmlStr, want)
				}
			}
		})
	}
}

func TestRenderEmptyState(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		message  string
		args     []string
		contains []string
	}{
		{
			name:     "basic empty state",
			title:    "No Items",
			message:  "Try creating one",
			contains: []string{"No Items", "Try creating one"},
		},
		{
			name:     "with check-circle icon",
			title:    "All Done",
			message:  "Nothing to do",
			args:     []string{"check-circle"},
			contains: []string{"All Done", "Nothing to do", "text-green-500"},
		},
		{
			name:     "with refresh icon",
			title:    "Loading",
			message:  "Please wait",
			args:     []string{"refresh"},
			contains: []string{"Loading", "Please wait", "text-blue-500"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := RenderEmptyState(tt.title, tt.message, tt.args...)
			htmlStr := string(html)

			for _, want := range tt.contains {
				if !strings.Contains(htmlStr, want) {
					t.Errorf("RenderEmptyState() = %v, want to contain %v", htmlStr, want)
				}
			}
		})
	}
}

func TestRenderPagination(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		contains []string
		empty    bool
	}{
		{
			name: "with PaginationData",
			data: PaginationData{
				CurrentPage:  2,
				TotalPages:   5,
				HasPrevious:  true,
				HasNext:      true,
				PreviousPage: 1,
				NextPage:     3,
				BaseURL:      "?page=",
			},
			contains: []string{"Page", "2", "5"},
		},
		{
			name: "with InstallsPageData",
			data: InstallsPageData{
				CurrentPage:  1,
				TotalPages:   3,
				HasPrevious:  false,
				HasNext:      true,
				PreviousPage: 0,
				NextPage:     2,
			},
			contains: []string{"Page", "1", "3"},
		},
		{
			name: "single page returns empty",
			data: PaginationData{
				CurrentPage: 1,
				TotalPages:  1,
			},
			empty: true,
		},
		{
			name:  "nil returns empty",
			data:  nil,
			empty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := RenderPagination(tt.data)
			htmlStr := string(html)

			if tt.empty {
				if htmlStr != "" {
					t.Errorf("RenderPagination() = %v, want empty string", htmlStr)
				}
				return
			}

			for _, want := range tt.contains {
				if !strings.Contains(htmlStr, want) {
					t.Errorf("RenderPagination() = %v, want to contain %v", htmlStr, want)
				}
			}
		})
	}
}

func TestMakePaginationData(t *testing.T) {
	pd := MakePaginationData(2, 10, true, true, 1, 3, "?tab=all&page=")

	if pd.CurrentPage != 2 {
		t.Errorf("CurrentPage = %v, want 2", pd.CurrentPage)
	}
	if pd.TotalPages != 10 {
		t.Errorf("TotalPages = %v, want 10", pd.TotalPages)
	}
	if !pd.HasPrevious {
		t.Error("HasPrevious = false, want true")
	}
	if !pd.HasNext {
		t.Error("HasNext = false, want true")
	}
	if pd.PreviousPage != 1 {
		t.Errorf("PreviousPage = %v, want 1", pd.PreviousPage)
	}
	if pd.NextPage != 3 {
		t.Errorf("NextPage = %v, want 3", pd.NextPage)
	}
	if pd.BaseURL != "?tab=all&page=" {
		t.Errorf("BaseURL = %v, want ?tab=all&page=", pd.BaseURL)
	}
}

func TestComponentsInTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmplStr  string
		data     map[string]interface{}
		contains []string
	}{
		{
			name:     "statusBadge in template",
			tmplStr:  `{{ statusBadge .Status }}`,
			data:     map[string]interface{}{"Status": "active"},
			contains: []string{"bg-green-100", "active"},
		},
		{
			name:     "statusBadge with args",
			tmplStr:  `{{ statusBadge .Status "install" "md" }}`,
			data:     map[string]interface{}{"Status": "pending"},
			contains: []string{"bg-blue-100", "pending", "text-sm"},
		},
		{
			name:     "alert in template",
			tmplStr:  `{{ alert "error" .ErrorMessage }}`,
			data:     map[string]interface{}{"ErrorMessage": "Something broke"},
			contains: []string{"bg-red-50", "Something broke"},
		},
		{
			name:     "emptyState in template",
			tmplStr:  `{{ emptyState "No data" "Check back later" "check-circle" }}`,
			data:     map[string]interface{}{},
			contains: []string{"No data", "Check back later", "text-green-500"},
		},
		{
			name:     "pagination in template",
			tmplStr:  `{{ pagination (paginationData 2 5 true true 1 3 "?page=") }}`,
			data:     map[string]interface{}{},
			contains: []string{"Page", "2", "5"},
		},
		{
			name:     "multiple components",
			tmplStr:  `<div>{{ statusBadge "active" }}{{ alert "info" "Hello" }}</div>`,
			data:     map[string]interface{}{},
			contains: []string{"bg-green-100", "active", "bg-blue-50", "Hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Funcs(getBaseFuncMap()).Parse(tt.tmplStr)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, tt.data); err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}

			result := buf.String()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("Template result = %v, want to contain %v", result, want)
				}
			}
		})
	}
}

func TestExtractPaginationFromMap(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want int // CurrentPage value to check
	}{
		{
			name: "camelCase keys",
			data: map[string]interface{}{
				"CurrentPage": 5,
				"TotalPages":  10,
			},
			want: 5,
		},
		{
			name: "snake_case keys",
			data: map[string]interface{}{
				"current_page": 3,
				"total_pages":  8,
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := extractPaginationFromMap(tt.data)
			if props.CurrentPage != tt.want {
				t.Errorf("extractPaginationFromMap() CurrentPage = %v, want %v", props.CurrentPage, tt.want)
			}
		})
	}
}
