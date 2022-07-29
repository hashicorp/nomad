// @ts-check

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class PoliciesNewController extends Controller {
  @service store;
  @service flashMessages;
  @service router;

  @action async savePolicy() {
    try {
      // NOTE: I don't understand why this is necessary
      this.model.id = this.model.name;

      await this.model.save();

      this.flashMessages.add({
        title: 'Policy Created!',
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });

      this.router.transitionTo('policies');
    } catch (error) {
      this.flashMessages.add({
        title: `Error creating Policy ${this.model.name}`,
        message: error,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
      throw error;
    }
  }
}
