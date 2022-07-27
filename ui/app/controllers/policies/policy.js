// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { set } from '@ember/object';
import { stringifyObject } from 'nomad-ui/helpers/stringify-object';
import { inject as service } from '@ember/service';

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
        title: `Error saving Policy ${this.model.path}`,
        message: error,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
    }
  }
}
