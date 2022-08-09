package resources

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Resource is a custom resource that users can configure to expose custom capabilities
// available per client.
type Resource struct {
	Name  string `hcl:"name,label"`
	Range *Range `hcl:"range,block,optional"`
	Set   *Set   `hcl:"set,block,optional"`
}

func (r *Resource) ValidateConfig() error {
	mErr := new(multierror.Error)

	// TODO: Should likely validate that only one type is set.

	if r.Range != nil {
		if err := r.Range.validateConfig(); err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("invalid config: resource %s of type range returned error - %s", r.Name, err.Error()))
		}
	}

	if r.Set != nil {
		if err := r.Set.validateConfig(); err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("invalid config: resource %s of type set returned error - %s", r.Name, err.Error()))
		}
	}

	return mErr.ErrorOrNil()
}

// Parse parses the resource spec from the given string.
func Parse(tmplContent string, filename string) (*Resource, error) {
	file, diags := hclsyntax.ParseConfig([]byte(tmplContent), filename, hcl.Pos{Byte: 0, Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("error parsing: %s", diags)
	}

	resourceContent, resourceDiags := file.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "resource",
				LabelNames: []string{"name"},
			},
		},
	})

	diags = append(diags, resourceDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	result := &Resource{}

	if len(resourceContent.Blocks) == 0 {
		return nil, fmt.Errorf("no resource found")
	}

	for _, block := range resourceContent.Blocks {
		if block.Type != "resource" {
			continue
		}

		result.Name = block.Labels[0]

		diags = append(diags, maybeParseRange(block, result)...)
		diags = append(diags, maybeParseSet(block, result)...)
		// Add parsing logic for all custom resource block types here, then trap
		// parsing errors once at the end of the loop. This should work because one
		// block should only target one resource type. The rest should be noops.
		if diags.HasErrors() {
			return nil, diags
		}
	}

	return result, nil
}

func appendDiag(diags hcl.Diagnostics, attribute *hcl.Attribute, block *hcl.Block, summary, detail string) hcl.Diagnostics {
	diags = append(diags, &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  summary,
		Detail:   detail,
		Subject:  &attribute.Range,
		Context:  &block.TypeRange,
	})
	return diags
}
