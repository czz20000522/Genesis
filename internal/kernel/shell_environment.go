package kernel

import (
	"os"
	"runtime"
	"strings"
)

func shellProcessEnvironment() []string {
	env := make([]string, 0, 16)
	seen := map[string]bool{}
	for _, raw := range os.Environ() {
		name, value, ok := strings.Cut(raw, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" || !shellEnvironmentCoreName(name) {
			continue
		}
		key := shellEnvironmentMapKey(name)
		if seen[key] {
			continue
		}
		if providerCommandEnvNameLooksSecret(name) || providerCommandEnvValueLooksSecret(value) {
			continue
		}
		seen[key] = true
		env = append(env, name+"="+value)
	}
	if runtime.GOOS == "windows" && !seen["pathext"] {
		env = append(env, "PATHEXT=.COM;.EXE;.BAT;.CMD")
	}
	return env
}

func shellEnvironmentCoreName(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "PATH",
		"PATHEXT",
		"SYSTEMROOT",
		"WINDIR",
		"COMSPEC",
		"TEMP",
		"TMP",
		"TMPDIR",
		"HOME",
		"USERPROFILE",
		"HOMEDRIVE",
		"HOMEPATH",
		"LANG",
		"LC_ALL",
		"LC_CTYPE",
		"TERM",
		"SHELL":
		return true
	default:
		return false
	}
}

func shellEnvironmentMapKey(name string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(name)
	}
	return name
}
