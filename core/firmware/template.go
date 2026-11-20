package firmware

import (
	"fmt"
	"strings"
)

func isVarChar(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

type tmplPart struct {
	text  string
	name  string
	isVar bool
}

func scanTemplate(tmpl string) []tmplPart {
	var parts []tmplPart
	last := 0
	i := 0
	for i < len(tmpl) {
		if tmpl[i] != '{' {
			i++
			continue
		}
		j := i + 1
		for j < len(tmpl) && isVarChar(tmpl[j]) {
			j++
		}
		if tmpl[i+1:min(j, len(tmpl))] == "env" && j+1 < len(tmpl) && tmpl[j] == '.' && isVarChar(tmpl[j+1]) {
			j++
			for j < len(tmpl) && isVarChar(tmpl[j]) {
				j++
			}
		}
		if j > i+1 && j < len(tmpl) && tmpl[j] == '}' {
			if i > last {
				parts = append(parts, tmplPart{text: tmpl[last:i]})
			}
			parts = append(parts, tmplPart{name: tmpl[i+1 : j], isVar: true})
			i = j + 1
			last = i
			continue
		}
		i++
	}
	if last < len(tmpl) {
		parts = append(parts, tmplPart{text: tmpl[last:]})
	}
	return parts
}

func Variables(tmpl string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range scanTemplate(tmpl) {
		if p.isVar && !seen[p.name] {
			seen[p.name] = true
			out = append(out, p.name)
		}
	}
	return out
}

const envPrefix = "env."

func IsEnvVar(name string) bool {
	return strings.HasPrefix(name, envPrefix)
}

func ExpandEnv(tmpl string, vars map[string]string, getenv func(string) string) (string, error) {
	merged := vars
	for _, name := range Variables(tmpl) {
		if !IsEnvVar(name) {
			continue
		}
		key := strings.TrimPrefix(name, envPrefix)
		v := getenv(key)
		if v == "" {
			return "", fmt.Errorf("environment variable %s is not set", key)
		}
		if len(merged) == len(vars) {
			merged = make(map[string]string, len(vars)+1)
			for k, x := range vars {
				merged[k] = x
			}
		}
		merged[name] = v
	}
	return Expand(tmpl, merged)
}

func Expand(tmpl string, vars map[string]string) (string, error) {
	var b strings.Builder
	for _, p := range scanTemplate(tmpl) {
		if !p.isVar {
			b.WriteString(p.text)
			continue
		}
		v, ok := vars[p.name]
		if !ok {
			return "", fmt.Errorf("unknown template variable {%s} in %q", p.name, tmpl)
		}
		b.WriteString(v)
	}
	return b.String(), nil
}
