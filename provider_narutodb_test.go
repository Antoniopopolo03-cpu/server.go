package main

import "testing"

func TestIsHTMLResponse(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		want        bool
	}{
		{
			name:        "html_content_type",
			contentType: "text/html; charset=utf-8",
			body:        `{"ok":true}`,
			want:        true,
		},
		{
			name:        "html_body_doctype",
			contentType: "application/json",
			body:        "<!DOCTYPE html><html><body>challenge</body></html>",
			want:        true,
		},
		{
			name:        "html_body_tag",
			contentType: "",
			body:        "<html><body>challenge</body></html>",
			want:        true,
		},
		{
			name:        "json_response",
			contentType: "application/json",
			body:        `{"characters":[{"name":"Naruto"}]}`,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHTMLResponse(tt.contentType, []byte(tt.body))
			if got != tt.want {
				t.Fatalf("isHTMLResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}
