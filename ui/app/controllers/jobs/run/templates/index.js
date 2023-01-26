import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { capitalize } from '@ember/string';

export default class JobsRunTemplatesIndexController extends Controller {
  @tracked selectedTemplate = null;

  get templates() {
    return this.model.map((templateVariable) => {
      const description = templateVariable.items.description;

      // Removes the preceeding nomad/job-templates/default/, as well as the namespace, from the ID
      let label;
      const delimiter = templateVariable.id.lastIndexOf('/');
      if (delimiter !== -1) {
        label = templateVariable.id.slice(delimiter + 1);
      } else {
        label = templateVariable.id;
      }

      label = capitalize(label.split('@')[0].replace(/-/g, ' '));

      return {
        id: templateVariable.id,
        label,
        description,
      };
    });
  }

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }
}
