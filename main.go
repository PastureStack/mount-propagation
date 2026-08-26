package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	// VERSION gets overridden at build time using -X main.VERSION=$VERSION
	VERSION       = "dev"
	cgroupPattern = regexp.MustCompile("^.*/docker-([a-z0-9]+).scope$")
	// Add the statepath as found on most OS's, and prefix with '/var' for Boot2Docker
	statePaths = []string{
		"/run/runc",
		"/var/run/runc",
		"/run/docker/execdriver/native",
		"/var/run/docker/execdriver/native",
		"/run/docker/runtime-runc/moby",
		"/var/run/docker/runtime-runc/moby",
		"/run/docker/runtime-nvidia/moby",
		"/var/run/docker/runtime-nvidia/moby",
	}
)

func main() {
	if err := newApp().Run(os.Args); err != nil {
		logrus.Fatal(safeLogValue(err))
	}
}

func newApp() *cli.App {
	app := cli.NewApp()
	app.Name = "mount-propagation"
	app.Usage = "PastureStack utility for shared mount propagation in a container namespace"
	app.Version = VERSION
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name: "stage2",
		},
	}
	app.Action = func(cli *cli.Context) {
		var fun cliFunc

		if cli.GlobalBool("stage2") {
			fun = stage2
		} else {
			fun = start
		}

		i, err := fun(cli)
		if err != nil {
			logrus.Fatal(safeLogValue(err))
		}
		os.Exit(i)
	}
	return app
}

type cliFunc func(cli *cli.Context) (int, error)

type State struct {
	ID             string `json:"id"`
	InitProcessPid int    `json:"init_process_pid"`
	Config         Config `json:"config"`
}

type Config struct {
	Rootfs string `json:"rootfs"`
}

type MountInfo struct {
	Mountpoint string
	Optional   string
}

func stage2(cli *cli.Context) (int, error) {
	mounts, err := getMounts()
	if err != nil {
		return -1, err
	}
	if rootIsShared(mounts) {
		logrus.Info("Root Directory is shared, skipping stage2")
		return 0, nil
	}

	for _, val := range cli.Args() {
		if val == "--" {
			break
		}

		if _, err := os.Stat(val); os.IsNotExist(err) {
			if err := os.MkdirAll(val, 0755); err != nil {
				return -1, err
			}
		}

		if err := makeShared(val); err != nil {
			logrus.Errorf("Failed to make shared %s: %s", safeLogValue(val), safeLogValue(err))
			return -1, err
		}
	}

	return 0, nil
}

func start(cli *cli.Context) (int, error) {
	paths := []string{}
	for _, i := range statePaths {
		paths = append(paths, filepath.Join("/host", i))
	}
	state, err := findState(paths...)
	if err != nil {
		return -1, err
	}

	mnt, err := getMntFd(state.InitProcessPid)
	if err != nil {
		return -1, err
	}

	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		return -1, err
	}

	nsenter, err := exec.LookPath("nsenter")
	if err != nil {
		logrus.Error("Failed to find nsenter: ", safeLogValue(err))
		return -1, err
	}

	args := []string{nsenter, "--mount=" + mnt, "-F", "--", path.Join(state.Config.Rootfs, self), "--stage2"}
	args = append(args, os.Args[1:]...)

	logrus.Infof("Execing %s", safeLogValue(args))
	return -1, syscall.Exec(nsenter, args, os.Environ())
}

func getMntFd(pid int) (string, error) {
	psStat := fmt.Sprintf("/proc/%d/stat", pid)
	content, err := os.ReadFile(psStat)
	if err != nil {
		return "", err
	}

	ppid, err := parseParentPID(string(content))
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", psStat, err)
	}
	return fmt.Sprintf("/proc/%d/ns/mnt", ppid), nil
}

func findContainerID() (string, error) {
	cgroupFile := fmt.Sprintf("/proc/%d/cgroup", os.Getpid())
	content, err := os.ReadFile(cgroupFile)
	if err != nil {
		return "", err
	}

	containerID, err := parseContainerID(strings.NewReader(string(content)))
	if err != nil {
		return "", fmt.Errorf("failed to find container id in %s: %w\n%s", cgroupFile, err, string(content))
	}
	return containerID, nil
}

func findState(stateRoots ...string) (*State, error) {
	containerID, err := findContainerID()
	if err != nil {
		containerID, err = findContainerIDFromState(stateRoots...)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println("Found container ID:", safeLogValue(containerID))
	return findStateByContainerID(containerID, stateRoots...)
}

func findStateByContainerID(containerID string, stateRoots ...string) (*State, error) {
	if containerID == "" {
		return nil, errors.New("container id must not be empty")
	}

	for _, stateRoot := range stateRoots {
		fmt.Println("Checking root:", safeLogValue(stateRoot))
		files, err := os.ReadDir(stateRoot)
		if err != nil {
			continue
		}

		for _, file := range files {
			fmt.Println("Checking file:", safeLogValue(file.Name()))
			if !strings.HasPrefix(file.Name(), containerID) {
				continue
			}

			bytes, err := os.ReadFile(path.Join(stateRoot, file.Name(), "state.json"))
			if err != nil {
				continue
			}

			fmt.Println("Found state.json:", safeLogValue(file.Name()))
			var state State
			return &state, json.Unmarshal(bytes, &state)
		}
	}

	return nil, errors.New("failed to find state.json")
}

func findContainerIDFromState(stateRoots ...string) (string, error) {
	pid := os.Getpid()
	containerID, err := findContainerIDFromStateForPID(pid, stateRoots...)
	if err == nil {
		return containerID, nil
	}

	content, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	return "", fmt.Errorf("failed to find container id from runtime state for pid %d: %w\n%s", pid, err, string(content))
}

func findContainerIDFromStateForPID(pid int, stateRoots ...string) (string, error) {
	for _, stateRoot := range stateRoots {
		files, err := os.ReadDir(stateRoot)
		if err != nil {
			continue
		}

		for _, file := range files {
			bytes, err := os.ReadFile(path.Join(stateRoot, file.Name(), "state.json"))
			if err != nil {
				continue
			}

			var state State
			if err := json.Unmarshal(bytes, &state); err != nil {
				continue
			}
			if state.InitProcessPid != pid {
				continue
			}

			if state.ID != "" {
				return state.ID, nil
			}
			return file.Name(), nil
		}
	}

	return "", errors.New("no matching runtime state")
}

func getMounts() ([]MountInfo, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseMountInfo(f)
}

func parseMountInfo(r io.Reader) ([]MountInfo, error) {
	mounts := []MountInfo{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}

		separator := -1
		for i, field := range fields {
			if field == "-" {
				separator = i
				break
			}
		}
		if separator < 0 || separator < 6 {
			continue
		}

		mounts = append(mounts, MountInfo{
			Mountpoint: fields[4],
			Optional:   strings.Join(fields[6:separator], " "),
		})
	}
	return mounts, scanner.Err()
}

func parseContainerID(r io.Reader) (string, error) {
	var systemdScopeID string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "docker/") && strings.Contains(line, ":devices:") {
			parts := strings.Split(line, "/")
			if containerID := parts[len(parts)-1]; containerID != "" {
				return containerID, nil
			}
		}

		if systemdScopeID == "" {
			matches := cgroupPattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				systemdScopeID = matches[1]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if systemdScopeID != "" {
		return systemdScopeID, nil
	}
	return "", errors.New("no Docker container id found")
}

func parseParentPID(stat string) (int, error) {
	commandEnd := strings.LastIndex(stat, ")")
	if commandEnd < 0 {
		return 0, errors.New("missing process command terminator")
	}

	fields := strings.Fields(stat[commandEnd+1:])
	if len(fields) < 2 {
		return 0, errors.New("missing parent process id")
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil || ppid <= 0 {
		return 0, fmt.Errorf("invalid parent process id %q", fields[1])
	}
	return ppid, nil
}

func rootIsShared(mounts []MountInfo) bool {
	for _, mount := range mounts {
		if mount.Mountpoint != "/" {
			continue
		}
		for _, optionalField := range strings.Fields(mount.Optional) {
			if strings.HasPrefix(optionalField, "shared") {
				return true
			}
		}
	}
	return false
}

func makeShared(target string) error {
	return syscall.Mount("", target, "", uintptr(syscall.MS_SHARED|syscall.MS_REC), "")
}

func safeLogValue(value interface{}) string {
	text := fmt.Sprint(value)
	text = strings.ReplaceAll(text, "\r", "")
	return strings.ReplaceAll(text, "\n", " ")
}
