import { getOwner } from '@ember/application';
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

export default class JobsRunTemplatesController extends Controller {
  @service router;
  @tracked selectedTemplate = null;

  get templates() {
    return this.model.map((templateVariable) => {
      // THIS LOGIC SHOULD LIKELY MOVE TO THE SERIALIZATION LAYER
      const { value: label } = templateVariable.keyValues.find((el) => {
        return el.key === 'label';
      });

      const { value: description } = templateVariable.keyValues.find((el) => {
        return el.key === 'description';
      });

      const { value: json } = templateVariable.keyValues.find((el) => {
        return el.key === 'json';
      });

      return {
        id: templateVariable.id,
        label,
        description,
        json,
      };
    });
  }

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }

  navigateToJobsRun(plainId) {
    return this.router.transitionTo('jobs.run', { queryParams: { plainId } });
  }

  @action
  async onApply() {
    const job = getOwner(this).lookup('controller:jobs.run.index').model;
    const { json } = this.templates.find(
      (template) => template.id === this.selectedTemplate
    );
    job.set('_newDefinition', json);
    await job.parse();
    this.navigateToJobsRun(job.plainId);
  }
}
