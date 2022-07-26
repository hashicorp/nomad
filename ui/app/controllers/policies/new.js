// @ts-check

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

export default class PoliciesNewController extends Controller {
  @service store;
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
}
