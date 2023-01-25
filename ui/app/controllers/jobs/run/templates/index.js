import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobsRunTemplatesIndexController extends Controller {
  @tracked selectedTemplate = null;

  get templates() {
    return this.model.map((templateVariable) => {
      // THIS LOGIC SHOULD LIKELY MOVE TO THE SERIALIZATION LAYER
      const description = templateVariable.items.description;
      return {
        id: templateVariable.id,
        label: templateVariable.path.split('nomad/job-templates/')[1],
        description,
      };
    });
  }

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }
}
