// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';

export default class PoliciesPolicyController extends Controller {
  @service flashMessages;
  @service router;

  @tracked isDeleting = false;

  @action
  onDeletePrompt() {
    this.isDeleting = true;
  }

  @action
  onDeleteCancel() {
    this.isDeleting = false;
  }

  @task(function* () {
    try {
      yield this.model.deleteRecord();
      yield this.model.save();
      this.flashMessages.add({
        title: 'Policy Deleted',
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });
      this.router.transitionTo('policies');
    } catch (err) {
      this.flashMessages.add({
        title: `Error deleting Policy ${this.model.name}`,
        message: err,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
    }
  })
  deletePolicy;
}
