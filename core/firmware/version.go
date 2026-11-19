package firmware

import "strings"

func versionParts(v string) []string {
	var parts []string
	i := 0
	for i < len(v) {
		if v[i] < '0' || v[i] > '9' {
			i++
			continue
		}
		j := i
		for j < len(v) && v[j] >= '0' && v[j] <= '9' {
			j++
		}
		parts = append(parts, strings.TrimLeft(v[i:j], "0"))
		i = j
	}
	return parts
}

func CompareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	n := max(len(pa), len(pb))
	for i := 0; i < n; i++ {
		var x, y string
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if len(x) != len(y) {
			if len(x) < len(y) {
				return -1
			}
			return 1
		}
		if c := strings.Compare(x, y); c != 0 {
			return c
		}
	}
	return 0
}

func InRange(version, since, until string) bool {
	if version == "" {
		return since == "" && until == ""
	}
	if since != "" && CompareVersions(version, since) < 0 {
		return false
	}
	if until != "" && CompareVersions(version, until) >= 0 {
		return false
	}
	return true
}

func (f Fact) Pick(version string) (Fact, bool) {
	if len(f.Alts) == 0 {
		if InRange(version, f.Since, f.Until) {
			return f, true
		}
		return Fact{}, false
	}
	for _, a := range f.Alts {
		if InRange(version, a.Since, a.Until) {
			return a, true
		}
	}
	return Fact{}, false
}
