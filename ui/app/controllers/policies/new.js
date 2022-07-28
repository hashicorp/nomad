// @ts-check

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class PoliciesNewController extends Controller {
  @service store;
  @service flashMessages;
  @service router;

  queryParams = ['path', 'view'];
  get existingPolicies() {
    return this.store.peekAll('policy');
  }

  //#region Code View
  /**
   * @type {"table" | "json"}
   */
  @tracked
  view = 'table';

  toggleView() {
    if (this.view === 'table') {
      this.view = 'json';
    } else {
      this.view = 'table';
    }
  }
  //#endregion Code View

  @action updatePolicy(value, codemirror) {
    this.model.rules = value;
  }

  @action async savePolicy() {
    try {
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
        title: `Error creating Policy ${this.model.path}`,
        message: error,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
      throw error;
    }
  }
}
