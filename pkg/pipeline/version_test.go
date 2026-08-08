// SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"path/filepath"
	"testing"

	"github.com/crossplane/upjet/v2/pkg/config"
)

func TestInsertPreviousObjectsMissingVersionDir(t *testing.T) {
	// The previous version's package directory does not exist on disk,
	// which is expected on the first run of the code generation pipeline
	// before any version of the group has been generated yet.
	vg := NewVersionGenerator(filepath.Join(t.TempDir(), "apis"), t.TempDir(), "github.com/upbound/provider-aws", "lakeformation.aws.upbound.io", "v1beta1")

	versions := map[string]map[string]*config.Resource{
		"v1beta2": {
			"aws_lakeformation_permissions": {
				Name:             "aws_lakeformation_permissions",
				Kind:             "Permissions",
				PreviousVersions: []string{"v1beta1"},
			},
		},
	}

	if err := vg.InsertPreviousObjects(versions); err != nil {
		t.Errorf("InsertPreviousObjects() should not error when the previous version's directory does not exist yet, got: %v", err)
	}
}
