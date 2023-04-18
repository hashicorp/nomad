import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';

export default class JobsRunTemplatesController extends Controller {
  @service flashMessages;
  @service router;
  @service system;

  @tracked formModalActive = false;

  @action
  updateKeyValue(key, value) {
    if (this.model.keyValues.find((kv) => kv.key === key)) {
      this.model.keyValues.find((kv) => kv.key === key).value = value;
    } else {
      this.model.keyValues.pushObject({ key, value });
    }
  }

  @action
  toggleModal() {
    this.formModalActive = !this.formModalActive;
  }

  @action
  async save(e, overwrite = false) {
    if (e.type === 'submit') {
      e.preventDefault();
    }

    try {
      await this.model.save({ adapterOptions: { overwrite } });

      this.flashMessages.add({
        title: 'Job template saved',
        message: `${this.model.path} successfully editted`,
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });

      this.router.transitionTo('jobs.run.templates');
    } catch (e) {
      this.flashMessages.add({
        title: 'Job template cannot be editted.',
        message: e,
        type: 'error',
        destroyOnClick: false,
        timeout: 5000,
      });
    }
  }

  @task(function* () {
    try {
      yield this.model.destroyRecord();

      this.flashMessages.add({
        title: 'Job template deleted',
        message: `${this.model.path} successfully deleted`,
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });
      this.router.transitionTo('jobs.run.templates.manage');
    } catch (err) {
      this.flashMessages.add({
        title: `Job template could not be deleted.`,
        message: err,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
    }
  })
  deleteTemplate;
}
