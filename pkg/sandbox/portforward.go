package sandbox

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ActiveForward represents an active port-forward session.
type ActiveForward struct {
	PID       int       `json:"pid"`
	Namespace string    `json:"namespace"`
	Sandbox   string    `json:"sandbox"`
	Ports     []string  `json:"ports"` // e.g. ["127.0.0.1:8080->80/TCP"]
	CreatedAt time.Time `json:"created_at"`
}

func forwardStateDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDir, "kampfire", "portforwards")
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

// FormatForwardedPort formats a local-to-remote port pair as a display string.
func FormatForwardedPort(local, remote int) string {
	return fmt.Sprintf("127.0.0.1:%d->%d/TCP", local, remote)
}

// NormalizePortPair parses "8080:80" or "8080" into local and remote integers.
func NormalizePortPair(p string) (local, remote int, err error) {
	parts := strings.Split(p, ":")
	if len(parts) == 1 {
		val, err := strconv.Atoi(parts[0])
		if err != nil || val <= 0 || val > 65535 {
			return 0, 0, fmt.Errorf("invalid port %q", p)
		}
		return val, val, nil
	} else if len(parts) == 2 {
		l, err1 := strconv.Atoi(parts[0])
		r, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || l <= 0 || l > 65535 || r <= 0 || r > 65535 {
			return 0, 0, fmt.Errorf("invalid port mapping %q", p)
		}
		return l, r, nil
	}
	return 0, 0, fmt.Errorf("invalid port format %q", p)
}

// RegisterActiveForward registers an active port-forward session in a local state file.
// Returns a cleanup function that deletes the state file when forwarding ends.
func RegisterActiveForward(namespace, sandbox string, rawPorts []string) (func(), error) {
	dir := forwardStateDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	var formatted []string
	for _, rp := range rawPorts {
		l, r, err := NormalizePortPair(rp)
		if err == nil {
			formatted = append(formatted, FormatForwardedPort(l, r))
		}
	}
	if len(formatted) == 0 {
		return func() {}, nil
	}

	pid := os.Getpid()
	filePath := filepath.Join(dir, fmt.Sprintf("%d.json", pid))

	af := ActiveForward{
		PID:       pid,
		Namespace: namespace,
		Sandbox:   sandbox,
		Ports:     formatted,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(af)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return nil, err
	}

	cleanup := func() {
		_ = os.Remove(filePath)
	}
	return cleanup, nil
}

// GetActiveForwards returns all actively forwarded ports mapped by sandbox name.
// It checks both the active state registry and running process table.
func GetActiveForwards(activeNamespace string) map[string][]string {
	result := make(map[string][]string)

	// 1. Read registered state files
	dir := forwardStateDir()
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			filePath := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var af ActiveForward
			if err := json.Unmarshal(data, &af); err != nil {
				_ = os.Remove(filePath)
				continue
			}

			if !processAlive(af.PID) {
				// Process is no longer running, purge stale state
				_ = os.Remove(filePath)
				continue
			}

			if activeNamespace == "" || af.Namespace == "" || af.Namespace == activeNamespace {
				result[af.Sandbox] = append(result[af.Sandbox], af.Ports...)
			}
		}
	}

	// 2. Scan running process table as fallback (detects existing sessions and kubectl port-forwards)
	procForwards := scanRunningPortForwards(activeNamespace)
	for sb, ports := range procForwards {
		for _, p := range ports {
			if !containsString(result[sb], p) {
				result[sb] = append(result[sb], p)
			}
		}
	}

	return result
}

// scanRunningPortForwards scans system processes for running port-forward commands.
func scanRunningPortForwards(activeNamespace string) map[string][]string {
	result := make(map[string][]string)

	cmd := exec.Command("ps", "-eo", "pid,command")
	out, err := cmd.Output()
	if err != nil {
		// Fallback for systems where 'command' is named 'args'
		cmd = exec.Command("ps", "-eo", "pid,args")
		out, err = cmd.Output()
		if err != nil {
			return result
		}
	}

	myPID := os.Getpid()
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid == myPID || pid <= 0 {
			continue
		}

		cmdArgs := fields[1:]
		ns, sb, ports, ok := parsePortForwardCommandLine(cmdArgs)
		if !ok {
			continue
		}

		if activeNamespace != "" && ns != "" && ns != activeNamespace {
			continue
		}

		for _, p := range ports {
			l, r, err := NormalizePortPair(p)
			if err == nil {
				formatted := FormatForwardedPort(l, r)
				if !containsString(result[sb], formatted) {
					result[sb] = append(result[sb], formatted)
				}
			}
		}
	}

	return result
}

// parsePortForwardCommandLine inspects a process command-line slice to detect port-forwarding invocations.
func parsePortForwardCommandLine(args []string) (ns, sandbox string, ports []string, ok bool) {
	if len(args) == 0 {
		return "", "", nil, false
	}

	bin := filepath.Base(args[0])
	isKampfire := strings.Contains(bin, "kampfire")
	isKubectl := strings.Contains(bin, "kubectl") || bin == "k"
	if !isKampfire && !isKubectl {
		return "", "", nil, false
	}

	var subcmdIdx = -1
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "port-forward" || arg == "forward" || arg == "pf" {
			subcmdIdx = i
			break
		}
		if arg == "-n" && i+1 < len(args) {
			ns = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--namespace=") {
			ns = strings.TrimPrefix(arg, "--namespace=")
		} else if arg == "--namespace" && i+1 < len(args) {
			ns = args[i+1]
			i++
		}
	}

	if subcmdIdx == -1 {
		return "", "", nil, false
	}

	// Remaining arguments after subcmd
	var remaining []string
	for i := subcmdIdx + 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-n" && i+1 < len(args) {
			ns = args[i+1]
			i++
			continue
		} else if strings.HasPrefix(arg, "--namespace=") {
			ns = strings.TrimPrefix(arg, "--namespace=")
			continue
		} else if arg == "--namespace" && i+1 < len(args) {
			ns = args[i+1]
			i++
			continue
		} else if strings.HasPrefix(arg, "-") {
			// Other flag
			continue
		}
		remaining = append(remaining, arg)
	}

	if len(remaining) < 2 {
		return "", "", nil, false
	}

	rawSandbox := remaining[0]
	// Handle pod/name syntax from kubectl
	rawSandbox = strings.TrimPrefix(rawSandbox, "pod/")
	rawSandbox = strings.TrimPrefix(rawSandbox, "pods/")
	sandbox = rawSandbox

	for _, p := range remaining[1:] {
		if _, _, err := NormalizePortPair(p); err == nil {
			ports = append(ports, p)
		}
	}

	if len(ports) == 0 {
		return "", "", nil, false
	}

	return ns, sandbox, ports, true
}

// FormatPortsWithActiveForwards merges static container ports with active port-forward sessions.
func FormatPortsWithActiveForwards(staticPorts string, activeForwards ...[]string) string {
	var active []string
	for _, afList := range activeForwards {
		for _, af := range afList {
			if !containsString(active, af) {
				active = append(active, af)
			}
		}
	}

	if len(active) == 0 {
		return staticPorts
	}

	// Parse static container ports to avoid duplicates
	var staticList []string
	if staticPorts != "" && staticPorts != "-" {
		for _, sp := range strings.Split(staticPorts, ",") {
			sp = strings.TrimSpace(sp)
			if sp != "" {
				staticList = append(staticList, sp)
			}
		}
	}

	// Map of containerPort -> active string
	var merged []string
	for _, act := range active {
		merged = append(merged, act)
	}

	// Add any static ports that are not actively forwarded
	for _, sp := range staticList {
		// e.g. "80/TCP" -> check if any active forward targets "->80/TCP"
		alreadyForwarded := false
		for _, act := range active {
			if strings.HasSuffix(act, "->"+sp) || strings.HasSuffix(act, "->"+strings.ToUpper(sp)) {
				alreadyForwarded = true
				break
			}
		}
		if !alreadyForwarded {
			merged = append(merged, sp)
		}
	}

	return strings.Join(merged, ", ")
}

func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
