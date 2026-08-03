// SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"os"
	"testing"

	"github.com/antchfx/htmlquery"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	xptest "github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"
)

func TestScrapeRepo(t *testing.T) {
	type args struct {
		config *ScrapeConfiguration
	}
	type want struct {
		err    error
		pmPath string
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ScrapeAWSResources": {
			reason: "Should successfully scrape AWS resource metadata",
			args: args{
				config: &ScrapeConfiguration{
					RepoPath:       "testdata/aws/r",
					CodeXPath:      `//code[@class="language-terraform" or @class="language-hcl"]/text()`,
					PreludeXPath:   `//text()[contains(., "description") and contains(., "subcategory")]`,
					FieldDocXPath:  `//ul/li//code[1]/text()`,
					ImportXPath:    `//code[@class="language-shell"]/text()`,
					FileExtensions: []string{".markdown"},
				},
			},
			want: want{
				pmPath: "testdata/aws/pm.yaml",
			},
		},
		"ScrapeAzureResources": {
			reason: "Should successfully scrape Azure resource metadata",
			args: args{
				config: &ScrapeConfiguration{
					RepoPath:       "testdata/azure/r",
					CodeXPath:      `//code[@class="language-terraform" or @class="language-hcl"]/text()`,
					PreludeXPath:   `//text()[contains(., "description") and contains(., "subcategory")]`,
					FieldDocXPath:  `//ul/li//code[1]/text()`,
					ImportXPath:    `//code[@class="language-shell"]/text()`,
					FileExtensions: []string{".markdown"},
				},
			},
			want: want{
				pmPath: "testdata/azure/pm.yaml",
			},
		},
		"ScrapeGCPResources": {
			reason: "Should successfully scrape GCP resource metadata",
			args: args{
				config: &ScrapeConfiguration{
					RepoPath:       "testdata/gcp/r",
					CodeXPath:      `//code[@class="language-terraform" or @class="language-hcl"]/text()`,
					PreludeXPath:   `//text()[contains(., "description") and contains(., "subcategory")]`,
					FieldDocXPath:  `//ul/li//code[1]/text()`,
					ImportXPath:    `//code[@class="language-shell"]/text()`,
					FileExtensions: []string{".markdown"},
				},
			},
			want: want{
				pmPath: "testdata/gcp/pm.yaml",
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pm := NewProviderMetadata("test-provider")
			err := pm.ScrapeRepo(tc.args.config)
			if diff := cmp.Diff(tc.want.err, err, xptest.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nScrapeRepo(error): -want, +got:\n%s", tc.reason, diff)
			}
			if err != nil {
				return
			}
			pmExpected := ProviderMetadata{}
			buff, err := os.ReadFile(tc.want.pmPath)
			if err != nil {
				t.Errorf("Failed to load expected ProviderMetadata from file: %s", tc.want.pmPath)
			}
			if err := yaml.Unmarshal(buff, &pmExpected); err != nil {
				t.Errorf("Failed to unmarshal expected ProviderMetadata from file: %s", tc.want.pmPath)
			}
			// upcoming cmp.Diff fails if
			// resources[*].examples[*].dependencies or
			// resources[*].examples[*].references is not present in the expected
			// metadata document (and is thus nil when decoded). One way to handle
			// this would be not to initialize them to empty maps/slices while
			// populating the `ProviderMetadata` struct but this is good to eliminate
			// nil checks elsewhere. Thus, for the test cases, instead of having to manually
			// initialize them in the testcase YAML documents, we do so programmatically below
			for _, r := range pmExpected.Resources {
				for eKey, e := range r.Examples {
					if e.Dependencies == nil {
						e.Dependencies = make(Dependencies)
					}
					if e.References == nil {
						e.References = make(map[string]string)
					}
					r.Examples[eKey] = e
				}
				if len(r.ImportStatements) == 0 {
					r.ImportStatements = nil
				}
			}
			if diff := cmp.Diff(&pmExpected, pm, cmpopts.IgnoreUnexported(fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\nScrapeRepo(ProviderConfig): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestScrapeFieldDocsNestedArgument(t *testing.T) {
	// In all cases below, a top-level argument and a nested block argument
	// share the same name ("my_variable") but have different descriptions.
	// The nested list is documented as a sub-bullet of its parent argument,
	// which is the common convention used across Terraform provider docs.
	cases := map[string]struct {
		reason   string
		markdown string
		want     map[string]string
	}{
		"TightList": {
			reason: "A nested sub-bullet list directly under a tightly-packed (no blank lines) parent list must be scoped under the parent argument's name.",
			markdown: `## Argument Reference

- ` + "`my_variable`" + ` - Description A.
- ` + "`my_list`" + ` - List of nested configs.
  - ` + "`my_variable`" + ` - Description B.
`,
			want: map[string]string{
				"my_variable":         "- Description A.",
				"my_list":             "- List of nested configs.",
				"my_list.my_variable": "- Description B.",
			},
		},
		"LooseList": {
			reason: "Goldmark renders every item of a top-level list as \"loose\" (wrapping each item's content in a <p>) if any sibling item is separated by a blank line; the nested sub-bullet list must still be scoped under the parent argument's name.",
			markdown: `## Argument Reference

- ` + "`my_variable`" + ` - Description A.

- ` + "`my_list`" + ` - List of nested configs.
  - ` + "`my_variable`" + ` - Description B.
`,
			want: map[string]string{
				"my_variable":         "- Description A.",
				"my_list":             "- List of nested configs.",
				"my_list.my_variable": "- Description B.",
			},
		},
		"MultiLevelNesting": {
			reason: "A doubly-nested sub-bullet list must be scoped under the full dotted path of its ancestor arguments.",
			markdown: `## Argument Reference

- ` + "`my_variable`" + ` - Description A.
- ` + "`my_list`" + ` - List of nested configs.
  - ` + "`nested_list`" + ` - List of doubly-nested configs.
    - ` + "`my_variable`" + ` - Description C.
`,
			want: map[string]string{
				"my_variable":                     "- Description A.",
				"my_list":                         "- List of nested configs.",
				"my_list.nested_list":             "- List of doubly-nested configs.",
				"my_list.nested_list.my_variable": "- Description C.",
			},
		},
		"AcceptedValuesList": {
			reason: "A sub-bullet list enumerating an attribute's accepted values (a quoted literal, not a valid attribute identifier) is structurally identical to a nested block's argument list, but must not be scoped under the parent argument's name.",
			markdown: `## Argument Reference

- ` + "`sandbox_type`" + ` (Required) Which sandbox to use for pods in the node pool.
    Accepted values are:

    - ` + "`\"gvisor\"`" + `: Pods run within a gVisor sandbox.
`,
			want: map[string]string{
				"sandbox_type": "(Required) Which sandbox to use for pods in the node pool.\nAccepted values are:",
				`"gvisor"`:     ": Pods run within a gVisor sandbox.",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buff bytes.Buffer
			if err := goldmark.Convert([]byte(tc.markdown), &buff); err != nil {
				t.Fatalf("failed to convert markdown: %v", err)
			}
			doc, err := htmlquery.Parse(&buff)
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			r := &Resource{}
			r.scrapeFieldDocs(doc, `//ul/li//code[1]/text()`)

			if diff := cmp.Diff(tc.want, r.ArgumentDocs); diff != "" {
				t.Errorf("\n%s\nscrapeFieldDocs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
