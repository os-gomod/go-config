package process

import (
	"bytes"
	"text/template"
)

// TemplateProcessor renders string templates.
type TemplateProcessor struct {
	funcs template.FuncMap
}

func NewTemplateProcessor() *TemplateProcessor {
	return &TemplateProcessor{funcs: template.FuncMap{}}
}

func (p *TemplateProcessor) AddFunc(name string, fn any) {
	p.funcs[name] = fn
}

func (p *TemplateProcessor) Process(data map[string]any) (map[string]any, error) {
	out := make(map[string]any)
	for k, v := range data {
		rendered, err := p.processValue(v, data)
		if err != nil {
			return nil, err
		}
		out[k] = rendered
	}
	return out, nil
}

func (p *TemplateProcessor) processValue(v any, data map[string]any) (any, error) {
	s, ok := v.(string)
	if !ok {
		return v, nil
	}

	return p.renderTemplate(s, data)
}

func (p *TemplateProcessor) renderTemplate(s string, data map[string]any) (string, error) {
	tpl, err := template.New("cfg").Funcs(p.funcs).Parse(s)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
