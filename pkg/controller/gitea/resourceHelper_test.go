package gitea

import (
	"testing"
)

func TestCreateResource(t *testing.T) {
	resourceHelper := newResourceHelper(&MockCR)
	resourceHelper.templateHelper.TemplatePath = "../../../template"

	for _, template := range Templates {
		obj, err := resourceHelper.createResource(template)

		if err != nil {
			t.Errorf("Error creating resource for template %s", template)
		}

		if obj == nil {
			t.Errorf("Invalid resource for template %s", template)
		}
	}
}
