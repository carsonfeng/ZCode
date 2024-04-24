package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/appleboy/com/file"
	"github.com/carsonfeng/ZCode/util"
)

var excludeFromDiff = []string{
	"package-lock.json",
	"pnpm-lock.yaml",
	// yarn.lock, Cargo.lock, Gemfile.lock, Pipfile.lock, etc.
	"*.lock",
	"go.sum",
}

type Command struct {
	// Generate diffs with <n> lines of context instead of the usual three
	diffUnified   int
	excludeList   []string
	isAmend       bool
	diffTagPrefix string // review latest two tags commit changes diff tags is grep by this string. If empty, ignore this option.
}

func (c *Command) excludeFiles() []string {
	var excludedFiles []string
	for _, f := range c.excludeList {
		excludedFiles = append(excludedFiles, ":(exclude,top)"+f)
	}
	return excludedFiles
}

// IsDiffTag judge whether to compare the differences between the latest two tags
func (c *Command) IsDiffTag() (is bool, tag1, tag2 string) {
	if c.diffTagPrefix != "" {
		is = true
		tagCmd := c.latestTwoTags(c.diffTagPrefix)
		output, err := tagCmd.Output()
		if err != nil {
			return false, "", ""
		}
		tags := strings.Split(string(output), " ")
		if len(tags) == 2 {
			tag1, tag2 = tags[0], tags[1]
		}
	}
	return
}

func (c *Command) latestTwoTags(tagGrepHead string) *exec.Cmd {

	cmdStr := fmt.Sprintf("git tag --sort=-creatordate | grep '^%s' | head -n 2 | tr '\\n' ' ' | sed 's/ $//'", tagGrepHead)

	return exec.Command("bash", "-c", cmdStr)
}

func (c *Command) diffNames() *exec.Cmd {
	args := []string{
		"diff",
		"--name-only",
	}

	if c.diffTagPrefix != "" {
		if is, tag1, tag2 := c.IsDiffTag(); is && tag1 != "" && tag2 != "" {
			args = append(args, tag1, tag2)
		}
	} else {
		if c.isAmend {
			args = append(args, "HEAD^", "HEAD")
		} else {
			args = append(args, "--staged")
		}
	}

	excludedFiles := c.excludeFiles()
	args = append(args, excludedFiles...)

	return exec.Command(
		"git",
		args...,
	)
}

func (c *Command) diffFiles() *exec.Cmd {
	args := []string{
		"diff",
		"--ignore-all-space",
		"--diff-algorithm=minimal",
		"--unified=" + strconv.Itoa(c.diffUnified),
	}

	if c.diffTagPrefix != "" {
		if is, tag1, tag2 := c.IsDiffTag(); is && tag1 != "" && tag2 != "" {
			args = append(args, tag1, tag2)
		}
	} else {
		if c.isAmend {
			args = append(args, "HEAD^", "HEAD")
		} else {
			args = append(args, "--staged")
		}
	}

	excludedFiles := c.excludeFiles()
	args = append(args, excludedFiles...)

	return exec.Command(
		"git",
		args...,
	)
}

func (c *Command) hookPath() *exec.Cmd {
	args := []string{
		"rev-parse",
		"--git-path",
		"hooks",
	}

	return exec.Command(
		"git",
		args...,
	)
}

func (c *Command) gitDir() *exec.Cmd {
	args := []string{
		"rev-parse",
		"--git-dir",
	}

	return exec.Command(
		"git",
		args...,
	)
}

func (c *Command) commit(val string) *exec.Cmd {
	args := []string{
		"commit",
		"--no-verify",
		"--signoff",
		fmt.Sprintf("--message=%s", val),
	}

	if c.isAmend {
		args = append(args, "--amend")
	}

	return exec.Command(
		"git",
		args...,
	)
}

func (c *Command) Commit(val string) (string, error) {
	output, err := c.commit(val).Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// GitDir to show the (by default, absolute) path of the git directory of the working tree.
func (c *Command) GitDir() (string, error) {
	output, err := c.gitDir().Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Diff compares the differences between two sets of data.
// It returns a string representing the differences and an error.
// If there are no differences, it returns an empty string and an error.
func (c *Command) DiffFiles() (string, error) {
	output, err := c.diffNames().Output()
	if err != nil {
		return "", err
	}
	if string(output) == "" {
		return "", errors.New("please add your staged changes using git add <files...>")
	}

	output, err = c.diffFiles().Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (c *Command) InstallHook() error {
	hookPath, err := c.hookPath().Output()
	if err != nil {
		return err
	}

	target := path.Join(strings.TrimSpace(string(hookPath)), HookPrepareCommitMessageTemplate)
	if file.IsFile(target) {
		return errors.New("hook file prepare-commit-msg exist.")
	}

	content, err := util.GetTemplateByBytes(HookPrepareCommitMessageTemplate, nil)
	if err != nil {
		return err
	}

	return os.WriteFile(target, content, 0o755)
}

func (c *Command) UninstallHook() error {
	hookPath, err := c.hookPath().Output()
	if err != nil {
		return err
	}

	target := path.Join(strings.TrimSpace(string(hookPath)), HookPrepareCommitMessageTemplate)
	if !file.IsFile(target) {
		return errors.New("hook file prepare-commit-msg is not exist.")
	}
	return os.Remove(target)
}

func New(opts ...Option) *Command {
	// Instantiate a new config object with default values
	cfg := &config{}

	// Loop through each option passed as argument and apply it to the config object
	for _, o := range opts {
		o.apply(cfg)
	}

	// Instantiate a new Command object with the configurations from the config object
	cmd := &Command{
		diffUnified: cfg.diffUnified,
		// Append the user-defined excludeList to the default excludeFromDiff
		excludeList:   append(excludeFromDiff, cfg.excludeList...),
		isAmend:       cfg.isAmend,
		diffTagPrefix: cfg.diffTagPrefix,
	}

	return cmd
}
