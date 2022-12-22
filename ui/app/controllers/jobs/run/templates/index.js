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
      const description = templateVariable?.keyValues?.find((el) => {
        return el.key === 'description';
      })?.value;

      const json = templateVariable?.keyValues?.find((el) => {
        return el.key === 'template';
      })?.value;

      return {
        id: templateVariable.id,
        label: templateVariable.path.split('nomad/job-templates/')[1],
        description,
        json,
      };
    });
  }

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }

  @action
  navigateToJobsRun(plainId) {
    return this.router.transitionTo('jobs.run', { queryParams: { plainId } });
  }

  @action
  onApply() {
    const job = getOwner(this).lookup('controller:jobs.run.index').model;
    const { json } = this.templates?.find(
      (template) => template.id === this.selectedTemplate
    );

    job.set('_newDefinition', json);
    job.setIdByPayload(json);
    this.navigateToJobsRun(job.plainId);
  }
}
