import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobsRunTemplatesIndexController extends Controller {
  @tracked selectedTemplate = null;

  get templates() {
    return this.model.map((templateVariable) => {
      // THIS LOGIC SHOULD LIKELY MOVE TO THE SERIALIZATION LAYER
      const description = templateVariable.keyValues.findBy('key', 'description')?.value;
      return {
        id: templateVariable.id,
        label: templateVariable.id.split('nomad/job-templates/')[1].split('@')[0].replace(/-/g, ' '),
        description,
      };
    });
  }

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }
}
