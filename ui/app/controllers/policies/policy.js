// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';

export default class PoliciesPolicyController extends Controller {
  @service flashMessages;
  @service router;
  modifiedRules = '';

  @action updatePolicy(value, codemirror) {
    this.modifiedRules = value;
  }

  @action async savePolicy() {
    try {
      this.model.rules = this.modifiedRules;
      await this.model.save();

      this.flashMessages.add({
        title: 'Policy Updated!',
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });
      this.router.transitionTo('policies');
    } catch (error) {
      this.flashMessages.add({
        title: `Error saving Policy ${this.model.name}`,
        message: error,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
    }
  }

  @tracked
  error = null;

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
      this.router.transitionTo('policies');
      // TODO: alert the user that the policy was successfully deleted
    } catch (err) {
      this.error = {
        title: 'Could Not Delete Policy',
        description: messageForError(err, 'delete policy'),
      };

      throw err;
    }
  })
  deletePolicy;
}
