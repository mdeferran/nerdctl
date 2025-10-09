/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package serviceparser

import (
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/containerd/nerdctl/v2/pkg/testutil"
)

func lastOf(ss []string) string {
	return ss[len(ss)-1]
}

func TestParseBuild(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test is not compatible with windows")
	}

	const dockerComposeYAML = `
services:
  foo:
    build: ./fooctx
    pull_policy: always
  bar:
    image: barimg
    pull_policy: build
    build:
      context: ./barctx
      target: bartgt
      labels:
        bar: baz
      secrets:
        - source: src_secret
          target: tgt_secret
        - simple_secret
        - absolute_secret
  baz:
    image: bazimg
    build:
      context: ./bazctx
      dockerfile_inline: |
       FROM random
secrets:
  src_secret:
    file: test_secret1
  simple_secret:
    file: test_secret2
  absolute_secret:
    file: /tmp/absolute_secret
`
	comp := testutil.NewComposeDir(t, dockerComposeYAML)
	defer comp.CleanUp()

	project, err := testutil.LoadProject(comp.YAMLFullPath(), comp.ProjectName(), nil)
	assert.NilError(t, err)

	fooSvc, err := project.GetService("foo")
	assert.NilError(t, err)

	foo, err := Parse(project, fooSvc)
	assert.NilError(t, err)

	t.Logf("foo: %+v", foo)
	assert.Equal(t, DefaultImageName(project.Name, "foo"), foo.Image)
	assert.Equal(t, false, foo.Build.Force)
	assert.Equal(t, project.RelativePath("fooctx"), lastOf(foo.Build.BuildArgs))

	barSvc, err := project.GetService("bar")
	assert.NilError(t, err)

	bar, err := Parse(project, barSvc)
	assert.NilError(t, err)

	t.Logf("bar: %+v", bar)
	assert.Equal(t, "barimg", bar.Image)
	assert.Equal(t, true, bar.Build.Force)
	assert.Equal(t, project.RelativePath("barctx"), lastOf(bar.Build.BuildArgs))
	assert.Assert(t, in(bar.Build.BuildArgs, "--target=bartgt"))
	assert.Assert(t, in(bar.Build.BuildArgs, "--label=bar=baz"))
	secretPath := project.WorkingDir
	assert.Assert(t, in(bar.Build.BuildArgs, "--secret=id=tgt_secret,src="+secretPath+"/test_secret1"))
	assert.Assert(t, in(bar.Build.BuildArgs, "--secret=id=simple_secret,src="+secretPath+"/test_secret2"))
	assert.Assert(t, in(bar.Build.BuildArgs, "--secret=id=absolute_secret,src=/tmp/absolute_secret"))

	bazSvc, err := project.GetService("baz")
	assert.NilError(t, err)

	baz, err := Parse(project, bazSvc)
	assert.NilError(t, err)

	t.Logf("baz: %+v", baz)
	t.Logf("baz.Build.BuildArgs: %+v", baz.Build.BuildArgs)
	t.Logf("baz.Build.DockerfileInline: %q", baz.Build.DockerfileInline)
	assert.Assert(t, func() bool {
		return strings.TrimSpace(baz.Build.DockerfileInline) == "FROM random"
	}())
}

func TestParseBuildSSH(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("test is not compatible with windows")
	}

	const dockerComposeYAML = `
services:
  sshtest:
    image: sshtestimg
    build:
      context: ./sshctx
      ssh:
        - default
  sshwithpath:
    image: sshwithpathimg
    build:
      context: ./sshpathctx
      ssh:
        - mykey=/path/to/key
`
	comp := testutil.NewComposeDir(t, dockerComposeYAML)
	defer comp.CleanUp()

	project, err := testutil.LoadProject(comp.YAMLFullPath(), comp.ProjectName(), nil)
	assert.NilError(t, err)

	// Test SSH with default
	sshTestSvc, err := project.GetService("sshtest")
	assert.NilError(t, err)

	sshTest, err := Parse(project, sshTestSvc)
	assert.NilError(t, err)

	t.Logf("sshtest: %+v", sshTest)
	t.Logf("sshtest.Build.BuildArgs: %+v", sshTest.Build.BuildArgs)
	assert.Assert(t, in(sshTest.Build.BuildArgs, "--ssh=default"))

	// Test SSH with custom path
	sshWithPathSvc, err := project.GetService("sshwithpath")
	assert.NilError(t, err)

	sshWithPath, err := Parse(project, sshWithPathSvc)
	assert.NilError(t, err)

	t.Logf("sshwithpath: %+v", sshWithPath)
	t.Logf("sshwithpath.Build.BuildArgs: %+v", sshWithPath.Build.BuildArgs)
	assert.Assert(t, in(sshWithPath.Build.BuildArgs, "--ssh=mykey=/path/to/key"))
}
