package stats

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// UserTraffic holds a snapshot of live Xray counters for one user.
type UserTraffic struct {
	Name     string
	Upload   int64
	Download int64
}

// QueryUserTraffic calls `xray api statsquery` and returns per-user traffic.
func QueryUserTraffic(ctx context.Context, xrayBinary, apiAddress string) ([]UserTraffic, error) {
	cmd := exec.CommandContext(ctx, xrayBinary, "api", "statsquery",
		fmt.Sprintf("--server=%s", apiAddress),
		"-pattern", "user>>>",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("xray api statsquery: %w: %s", err, string(out))
	}
	return ParseStatsOutput(string(out)), nil
}

var (
	nameRe  = regexp.MustCompile(`name:\s*"([^"]+)"`)
	valueRe = regexp.MustCompile(`value:\s*(\d+)`)
)

// ParseStatsOutput parses the protobuf text-format output of `xray api statsquery`.
func ParseStatsOutput(output string) []UserTraffic {
	byUser := make(map[string]*UserTraffic)
	var currentName string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if m := nameRe.FindStringSubmatch(line); len(m) > 1 {
			currentName = m[1]
			continue
		}
		if m := valueRe.FindStringSubmatch(line); len(m) > 1 && currentName != "" {
			value, _ := strconv.ParseInt(m[1], 10, 64)
			parts := strings.Split(currentName, ">>>")
			if len(parts) == 4 && parts[0] == "user" && parts[2] == "traffic" {
				userName := parts[1]
				direction := parts[3]
				ut, ok := byUser[userName]
				if !ok {
					ut = &UserTraffic{Name: userName}
					byUser[userName] = ut
				}
				switch direction {
				case "uplink":
					ut.Upload = value
				case "downlink":
					ut.Download = value
				}
			}
			currentName = ""
		}
	}

	result := make([]UserTraffic, 0, len(byUser))
	for _, ut := range byUser {
		result = append(result, *ut)
	}
	return result
}
