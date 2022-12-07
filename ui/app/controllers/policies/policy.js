// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';

export default class PoliciesPolicyController extends Controller {
  @service flashMessages;
  @service router;

  @alias('model.policy') policy;
  @alias('model.tokens') tokens;

  @tracked
  error = null;

  @tracked isDeleting = false;

  get newTokenString() {
    return `nomad acl token create -name="<TOKEN_NAME>" -policy=${this.policy.name} -type=client -ttl=<8h>`
  }

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
      yield this.policy.deleteRecord();
      yield this.policy.save();
      this.flashMessages.add({
        title: 'Policy Deleted',
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });
      this.router.transitionTo('policies');
    } catch (err) {
      this.flashMessages.add({
        title: `Error deleting Policy ${this.policy.name}`,
        message: err,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
    }
  })
  deletePolicy;

  @task(function* () {
    try {
      const newToken = this.store.createRecord('token', {
        name: `Example Token for ${this.policy.name}`,
        policies: [this.policy],
        // New date 10 minutes into the future
        expirationTime: new Date(Date.now() + 10 * 60 * 1000),
        type: 'client',
      });
      yield newToken.save();
      console.table(newToken.toJSON());
      console.log('Accessor:', newToken.accessor);
      this.flashMessages.add({
        title: 'Example Token Created',
        message: `${newToken.accessor}`,
        type: 'success',
        destroyOnClick: false,
        timeout: 60000,
      });
    } catch (err) {
      this.error = {
        title: 'Error creating new token',
        description: err,
      };

      throw err;
    }
  })
  createTestToken;
}
