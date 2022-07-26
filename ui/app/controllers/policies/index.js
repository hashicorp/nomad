import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class PoliciesIndexController extends Controller {
  @service router;
  @service store;

  isForbidden = false;

  @action
  goToPolicy(policy) {
    this.router.transitionTo('policies.policy', policy.path);
  }

  @controller policies;

  get hasPolicies() {
    return this.model.policies.length;
  }

  get root() {
    return this.model.root;
  }
}
