package muos

import "strings"

func HiddenLayout(fsName string, m3u []byte) (string, []byte, bool) {
	folder := "_" + fsName
	if m3u == nil {
		return folder, nil, true
	}
	lines := strings.Split(string(m3u), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			line = folder + "/" + line
		}
		lines[i] = line
	}
	return folder, []byte(strings.Join(lines, "\n")), true
}
