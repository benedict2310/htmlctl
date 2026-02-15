package renderer

import (
	"bytes"
	"fmt"
	"text/template"
)

type pageTemplateData struct {
	Title       string
	Description string
	StyleHrefs  []string
	ContentHTML string
	ScriptSrc   string
}

const defaultPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <meta name="description" content="{{.Description}}">
{{- range .StyleHrefs }}
  <link rel="stylesheet" href="{{.}}">
{{- end }}
</head>
<body>
  <main>
{{.ContentHTML}}
  </main>
{{- if .ScriptSrc }}
  <script src="{{.ScriptSrc}}"></script>
{{- end }}
</body>
</html>
`

func renderDefaultTemplate(data pageTemplateData) ([]byte, error) {
	tmpl, err := template.New("default").Parse(defaultPageTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse default template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute default template: %w", err)
	}

	return normalizeLFBytes(buf.Bytes()), nil
}
