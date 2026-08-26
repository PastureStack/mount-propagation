package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIHelpUsesPastureStackNameWithoutRunningMountCode(t *testing.T) {
	app := newApp()
	var output bytes.Buffer
	app.Writer = &output
	app.ErrWriter = &output

	if err := app.Run([]string{"mount-propagation", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "mount-propagation") {
		t.Fatalf("help output did not contain the primary executable name: %q", output.String())
	}
	if !strings.Contains(output.String(), "PastureStack") {
		t.Fatalf("help output did not identify the PastureStack project: %q", output.String())
	}
}

func TestParseContainerIDPrefersLegacyDevicesEntry(t *testing.T) {
	cgroups := strings.NewReader(strings.Join([]string{
		"0::/system.slice/docker-systemd123.scope",
		"11:devices:/docker/legacy456",
	}, "\n"))

	containerID, err := parseContainerID(cgroups)
	if err != nil {
		t.Fatal(err)
	}
	if containerID != "legacy456" {
		t.Fatalf("container id = %q, want legacy456", containerID)
	}
}

func TestParseContainerIDSupportsSystemdScope(t *testing.T) {
	containerID, err := parseContainerID(strings.NewReader("0::/system.slice/docker-abcdef012345.scope\n"))
	if err != nil {
		t.Fatal(err)
	}
	if containerID != "abcdef012345" {
		t.Fatalf("container id = %q, want abcdef012345", containerID)
	}
}

func TestParseContainerIDRejectsHostCgroup(t *testing.T) {
	if _, err := parseContainerID(strings.NewReader("0::/user.slice/user-1000.slice\n")); err == nil {
		t.Fatal("expected a cgroup without a Docker container id to fail")
	}
}

func TestParseParentPIDHandlesSpacesAndParenthesesInCommand(t *testing.T) {
	parentPID, err := parseParentPID("123 (worker (mount helper)) S 77 1 1 0")
	if err != nil {
		t.Fatal(err)
	}
	if parentPID != 77 {
		t.Fatalf("parent pid = %d, want 77", parentPID)
	}
}

func TestParseParentPIDRejectsMalformedStat(t *testing.T) {
	if _, err := parseParentPID("123 malformed"); err == nil {
		t.Fatal("expected malformed stat data to fail")
	}
}

func TestParseMountInfoAndSharedRoot(t *testing.T) {
	mountInfo := strings.NewReader(strings.Join([]string{
		"36 25 0:32 / / rw,relatime shared:1 master:2 - ext4 /dev/root rw",
		"37 36 0:33 / /var/lib rw,relatime - ext4 /dev/root rw",
		"malformed",
	}, "\n"))

	mounts, err := parseMountInfo(mountInfo)
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 2 {
		t.Fatalf("parsed %d mounts, want 2", len(mounts))
	}
	if mounts[0].Mountpoint != "/" || mounts[0].Optional != "shared:1 master:2" {
		t.Fatalf("unexpected root mount: %#v", mounts[0])
	}
	if !rootIsShared(mounts) {
		t.Fatal("expected root mount to be detected as shared")
	}
}

func TestFindStateByContainerIDUsesTemporaryRuntimeState(t *testing.T) {
	stateRoot := t.TempDir()
	stateDir := filepath.Join(stateRoot, "abcdef0123456789")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateJSON := `{"id":"abcdef0123456789","init_process_pid":4242,"config":{"rootfs":"/runtime/rootfs"}}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(stateJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	state, err := findStateByContainerID("abcdef", stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if state.ID != "abcdef0123456789" || state.InitProcessPid != 4242 || state.Config.Rootfs != "/runtime/rootfs" {
		t.Fatalf("unexpected runtime state: %#v", state)
	}
}

func TestFindContainerIDFromStateForPIDFallsBackToDirectoryName(t *testing.T) {
	stateRoot := t.TempDir()
	stateDir := filepath.Join(stateRoot, "directory-container-id")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stateJSON := `{"init_process_pid":5150,"config":{"rootfs":"/runtime/rootfs"}}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(stateJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	containerID, err := findContainerIDFromStateForPID(5150, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if containerID != "directory-container-id" {
		t.Fatalf("container id = %q, want directory-container-id", containerID)
	}
}

func TestSafeLogValueProducesSingleRecord(t *testing.T) {
	if got := safeLogValue("first\r\nforged\nthird"); got != "first forged third" {
		t.Fatalf("unexpected safe log value: %q", got)
	}
}
