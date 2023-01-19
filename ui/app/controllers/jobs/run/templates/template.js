import { A } from '@ember/array';
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobsRunTemplatesController extends Controller {
  @service router;
  @service system;

  @tracked formModalActive = false;

  keyValues = A([]);

  @action
  updateKeyValue(key, value) {
    if (this.keyValues.find((kv) => kv.key === key)) {
      this.keyValues.find((kv) => kv.key === key).value = value;
    } else {
      this.keyValues.pushObject({ key, value });
    }
  }

  @action
  toggleModal() {
    this.formModalActive = !this.formModalActive;
  }

  @action
  async deleteTemplateAndClose() {
    try {
      this.toggleModal();
      await this.model.destroyRecord();

      this.flashMessages.add({
        title: 'Job template deleted',
        message: `${this.templateName} successfully deleted`,
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });

      this.router.transitionTo('jobs.run.templates');
    } catch (e) {
      this.flashMessages.add({
        title: 'Job template could not be deleted.',
        message: e,
        type: 'error',
        destroyOnClick: false,
        timeout: 5000,
      });
    }
  }

  @action
  async save(e, overwrite = false) {
    if (e.type === 'submit') {
      e.preventDefault();
    }

    this.model.set('keyValues', this.keyValues);

    try {
      await this.model.save({ adapterOptions: { overwrite } });

      this.flashMessages.add({
        title: 'Job template saved',
        message: `${this.templateName} successfully editted`,
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
}
