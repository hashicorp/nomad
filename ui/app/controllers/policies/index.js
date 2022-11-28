import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { alias } from '@ember/object/computed';

export default class PoliciesIndexController extends Controller {
  @service router;
  // @alias('model.policies') policies;
  // @alias('model.tokens') tokens;
  get policies() {
    return this.model.policies.map((policy) => {
      policy.tokens = this.model.tokens.filter((token) => {
        return token.policies.includes(policy);
      });
      return policy;
    });
  }

  @action openPolicy(policy) {
    this.router.transitionTo('policies.policy', policy.name);
  }
}
