//
// Copyright (C) 2025 The Android Open Source Project
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
	"runtime"

	"android/soong/android"
	"android/soong/rust"
	"android/soong/rust/config"

	"github.com/google/blueprint/proptools"
)

// This module contains logic for defining file groups inside of a Rust
// toolchain.  It differs from rust_stdlib_prebuilt_filegroup_host by searching
// from the root of the current toolchain directory instead of the host's
// target specific directory.
func init() {
	android.RegisterModuleType("rust_toolchain_filegroup", rustToolchainFileGroupFactory)
}

type toolchainFilegroupProps struct {
	Toolchain_srcs []string
}

func rustToolchainFileGroupFactory() android.Module {
	module := android.FileGroupFactory()

	p := &toolchainFilegroupProps{}
	module.AddProperties(p)

	android.AddLoadHook(module, func(ctx android.LoadHookContext) {
		rustSetToolchainFileGroupSource(ctx, p)
	})

	return module
}

func rustSetToolchainFileGroupSource(ctx android.LoadHookContext, inProps *toolchainFilegroupProps) {
	type props struct {
		Srcs    []string
		Enabled *bool
	}

	p := &props{}
	if runtime.GOOS == "linux" {
		p.Enabled = proptools.BoolPtr(true)
	} else {
		p.Enabled = proptools.BoolPtr(false)
	}

	prebuiltsDir := path.Join(config.HostPrebuiltTag(ctx.Config()), rust.GetRustPrebuiltVersion(ctx))
	for _, src := range inProps.Toolchain_srcs {
		p.Srcs = append(p.Srcs, path.Join(prebuiltsDir, src))
	}
	ctx.AppendProperties(p)
}
