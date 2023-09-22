//
// Copyright (C) 2021 The Android Open Source Project
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package rustprebuilts

import (
	"path"
	"strings"

	"github.com/google/blueprint/proptools"

	"android/soong/android"
	"android/soong/cc"
	"android/soong/rust"
	"android/soong/rust/config"
)

// This module is used to generate the rust host stdlib prebuilts
// When RUST_PREBUILTS_VERSION is set, the library will generated
// from the given Rust version.
func init() {
	android.RegisterModuleType("rust_stdlib_prebuilt_host",
		rustHostPrebuiltSysrootLibraryFactory)
	android.RegisterModuleType("rust_stdlib_prebuilt_rlib_host",
		rustHostPrebuiltSysrootRlibFactory)
	android.RegisterModuleType("rust_stdlib_prebuilt_static_lib_host",
		rustHostPrebuiltSysrootStaticLibFactory)
}

func getRustPrebuiltVersion(ctx android.LoadHookContext) string {
	return ctx.AConfig().GetenvWithDefault("RUST_PREBUILTS_VERSION", config.RustDefaultVersion)
}

func getRustLibDir(ctx android.LoadHookContext) string {
	rustDir := getRustPrebuiltVersion(ctx)
	return path.Join(rustDir, "lib", "rustlib")
}

// getPrebuilt returns the module relative Rust library path and the suffix hash.
func getPrebuilt(ctx android.LoadHookContext, dir, lib, extension string) (string, string) {
	globPath := path.Join(ctx.ModuleDir(), dir, lib) + "-*" + extension
	libMatches := ctx.Glob(globPath, nil)

	if len(libMatches) != 1 {
		ctx.ModuleErrorf("Unexpected number of matches for prebuilt libraries at path %q, found %d matches", globPath, len(libMatches))
		return "", ""
	}

	// Collect the suffix by trimming the extension from the Base, then removing the library name and hyphen.
	suffix := strings.TrimSuffix(libMatches[0].Base(), extension)[len(lib)+1:]

	// Get the relative path from the match by trimming out the module directory.
	relPath := strings.TrimPrefix(libMatches[0].String(), ctx.ModuleDir()+"/")

	return relPath, suffix
}

type targetProps struct {
	Suffix *string
	Dylib  struct {
		Srcs []string
	}
	Rlib struct {
		Srcs []string
	}
	Link_dirs []string
	Enabled   *bool
}

type props struct {
	Enabled *bool
	Target  struct {
		Linux_glibc_x86_64 targetProps
		Linux_glibc_x86    targetProps
		Linux_musl_x86_64  targetProps
		Linux_musl_x86     targetProps
		Darwin_x86_64      targetProps
	}
}

func (target *targetProps) addPrebuiltToTarget(ctx android.LoadHookContext, libName, rustDir, platform, arch string, rlib, solib bool) {
	dir := path.Join(platform, rustDir, arch, "lib")
	target.Link_dirs = []string{dir}
	target.Enabled = proptools.BoolPtr(true)
	if rlib {
		rlib, suffix := getPrebuilt(ctx, dir, libName, ".rlib")
		target.Rlib.Srcs = []string{rlib}
		target.Suffix = proptools.StringPtr(suffix)
	}
	if solib {
		// The suffixes are the same between the dylib and the rlib,
		// so it's okay if we overwrite the rlib suffix
		dylib, suffix := getPrebuilt(ctx, dir, libName, ".so")
		target.Dylib.Srcs = []string{dylib}
		target.Suffix = proptools.StringPtr(suffix)
	}
}

func constructLibProps(rlib, solib bool) func(ctx android.LoadHookContext) {
	return func(ctx android.LoadHookContext) {
		rustDir := getRustLibDir(ctx)
		name := android.RemoveOptionalPrebuiltPrefix(ctx.ModuleName())
		name = strings.Replace(name, ".rust_sysroot", "", -1)

		p := props{}
		p.Enabled = proptools.BoolPtr(false)

		if ctx.Config().BuildOS == android.Linux {
			p.Target.Linux_glibc_x86_64.addPrebuiltToTarget(ctx, name, rustDir, "linux-x86", "x86_64-unknown-linux-gnu", rlib, solib)
			p.Target.Linux_glibc_x86.addPrebuiltToTarget(ctx, name, rustDir, "linux-x86", "i686-unknown-linux-gnu", rlib, solib)
		} else if ctx.Config().BuildOS == android.LinuxMusl {
			p.Target.Linux_musl_x86_64.addPrebuiltToTarget(ctx, name, rustDir, "linux-musl-x86", "x86_64-unknown-linux-musl", rlib, solib)
			p.Target.Linux_musl_x86.addPrebuiltToTarget(ctx, name, rustDir, "linux-musl-x86", "i686-unknown-linux-musl", rlib, solib)
		} else if ctx.Config().BuildOS == android.Darwin {
			p.Target.Darwin_x86_64.addPrebuiltToTarget(ctx, name, rustDir, "darwin-x86", "x86_64-apple-darwin", rlib, solib)
		}

		ctx.AppendProperties(&p)
	}
}

func constructStaticLibProps(ctx android.LoadHookContext) {
	rustDir := getRustLibDir(ctx)
	name := android.RemoveOptionalPrebuiltPrefix(ctx.ModuleName())
	name = strings.Replace(name, ".rust_sysroot_static", "", -1)

	type ccTargetProps struct {
		Enabled *bool
		Srcs    []string
	}
	p := struct {
		Enabled *bool
		Target  struct {
			Linux_glibc_x86_64 ccTargetProps
			Linux_glibc_x86    ccTargetProps
			Linux_musl_x86_64  ccTargetProps
			Linux_musl_x86     ccTargetProps
			Darwin_x86_64      ccTargetProps
		}
	}{}
	p.Enabled = proptools.BoolPtr(false)

	addPrebuiltToTarget := func(platform, arch string) ccTargetProps {
		lib := path.Join(platform, rustDir, arch, "lib", name+".a")
		return ccTargetProps{
			Enabled: proptools.BoolPtr(true),
			Srcs:    []string{lib},
		}
	}

	if ctx.Config().BuildOS == android.Linux {
		p.Target.Linux_glibc_x86_64 = addPrebuiltToTarget("linux-x86", "x86_64-unknown-linux-gnu")
		p.Target.Linux_glibc_x86 = addPrebuiltToTarget("linux-x86", "i686-unknown-linux-gnu")
	} else if ctx.Config().BuildOS == android.LinuxMusl {
		p.Target.Linux_musl_x86_64 = addPrebuiltToTarget("linux-musl-x86", "x86_64-unknown-linux-musl")
		p.Target.Linux_musl_x86 = addPrebuiltToTarget("linux-musl-x86", "i686-unknown-linux-musl")
	} else if ctx.Config().BuildOS == android.Darwin {
		p.Target.Darwin_x86_64 = addPrebuiltToTarget("darwin-x86", "x86_64-apple-darwin")
	}

	ctx.AppendProperties(&p)
}

func rustHostPrebuiltSysrootLibraryFactory() android.Module {
	module, _ := rust.NewPrebuiltLibrary(android.HostSupported)
	android.AddLoadHook(module, constructLibProps( /*rlib=*/ true /*solib=*/, true))
	return module.Init()
}

func rustHostPrebuiltSysrootRlibFactory() android.Module {
	module, _ := rust.NewPrebuiltRlib(android.HostSupported)
	android.AddLoadHook(module, constructLibProps( /*rlib=*/ true /*solib=*/, false))
	return module.Init()
}

func rustHostPrebuiltSysrootStaticLibFactory() android.Module {
	module, _ := cc.NewPrebuiltStaticLibrary(android.HostSupported)
	android.AddLoadHook(module, constructStaticLibProps)
	return module.Init()
}
