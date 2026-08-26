package main

import (
	"strings"
	"testing"

	"github.com/agtorre/gocolorize"
	"github.com/jessevdk/go-flags"
	"github.com/revel/cmd/model"
	"github.com/revel/cmd/model/command"
)

// Frozen CLI parsing contract for the vendored Revel tool (go-flags v1.4.0
// semantics on 2026-08-26). Upgrading go-flags to v1.6.1 must keep the
// production invocation `build [-v] <import path> <target>` behaving exactly
// the same.
func TestParseBuildInvocation(t *testing.T) {
	c := &model.CommandConfig{}
	parser := flags.NewParser(c, flags.HelpFlag|flags.PassDoubleDash)
	extra, err := parser.ParseArgs([]string{"build", "-v", "../..", "./tmptmp"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if parser.Active == nil || parser.Active.Name != "build" {
		t.Fatalf("active command = %v, want build", parser.Active)
	}
	if len(c.Verbose) != 1 {
		t.Errorf("Verbose = %v, want one -v", c.Verbose)
	}
	if len(extra) != 2 || extra[0] != "../.." || extra[1] != "./tmptmp" {
		t.Fatalf("extra args = %v, want [../.. ./tmptmp]", extra)
	}

	if !updateBuildConfig(c, extra) {
		t.Fatal("updateBuildConfig returned false")
	}
	if c.Index != model.BUILD {
		t.Errorf("Index = %d, want BUILD (%d)", c.Index, model.BUILD)
	}
	if c.Build.ImportPath != "../.." || c.Build.TargetPath != "./tmptmp" {
		t.Errorf("build config = %q/%q, want ../.. ./tmptmp", c.Build.ImportPath, c.Build.TargetPath)
	}
	if c.Build.Mode != "" {
		t.Errorf("Mode = %q, want empty (defaults applied later)", c.Build.Mode)
	}
}

func TestUpdateBuildConfigDefaults(t *testing.T) {
	// the pre-configured import path lives on Build.ImportPath (embedded
	// ImportCommand), matching how buildApp consumes it
	c := &model.CommandConfig{
		Build: command.Build{ImportCommand: command.ImportCommand{ImportPath: "github.com/leanote/leanote"}},
	}
	if !updateBuildConfig(c, nil) {
		t.Fatal("pre-configured import path with no args must succeed")
	}
	if c.Build.TargetPath != "target" {
		t.Errorf("default TargetPath = %q, want target", c.Build.TargetPath)
	}

	c2 := &model.CommandConfig{}
	if updateBuildConfig(c2, []string{"only-one-arg"}) {
		t.Error("single positional arg must fail")
	}
}

func TestGocolorizePlainContract(t *testing.T) {
	defer gocolorize.SetPlain(false)

	gocolorize.SetPlain(false)
	colored := gocolorize.Colorize{Fg: gocolorize.Red}.Paint("x")
	if !strings.Contains(colored, "\x1b[") {
		t.Errorf("plain=false output %q has no ANSI escape", colored)
	}

	gocolorize.SetPlain(true)
	plainOut := gocolorize.Colorize{Fg: gocolorize.Red}.Paint("x")
	if strings.Contains(plainOut, "\x1b[") {
		t.Errorf("plain=true output %q still has ANSI escapes", plainOut)
	}
	if plainOut != "x" {
		t.Errorf("plain=true output = %q, want x", plainOut)
	}
}
