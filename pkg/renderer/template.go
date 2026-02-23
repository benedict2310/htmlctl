package renderer

import (
	"bytes"
	"fmt"
	"html/template"
)

type pageTemplateData struct {
	Title        string
	Description  string
	HeadMetaHTML template.HTML
	StyleHrefs   []string
	// ContentHTML is trusted component markup; all other fields remain auto-escaped.
	ContentHTML template.HTML
	ScriptSrc   string
}

const defaultPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <meta name="description" content="{{.Description}}">
{{- if .HeadMetaHTML }}
{{.HeadMetaHTML}}{{- end }}
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

var defaultTmpl = template.Must(template.New("default").Parse(defaultPageTemplate))

func renderDefaultTemplate(data pageTemplateData) ([]byte, error) {
	var buf bytes.Buffer
	if err := defaultTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute default template: %w", err)
	}

	return normalizeLFBytes(buf.Bytes()), nil
}
