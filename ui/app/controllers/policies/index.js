import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

const ALL_NAMESPACE_WILDCARD = '*';

export default class PoliciesIndexController extends Controller {
  @service router;
  @service store;

  isForbidden = false;

  @action
  goToPolicy(policy) {
    this.router.transitionTo('policies.policy', policy.path);
  }

  @controller policies;

  @action
  setNamespace(namespace) {
    this.policies.setNamespace(namespace);
  }

  get namespaceSelection() {
    return this.policies.qpNamespace;
  }

  get hasPolicies() {
    return this.model.policies.length;
  }

  get root() {
    return this.model.root;
  }

  get namespaceOptions() {
    const namespaces = this.store
      .peekAll('namespace')
      .map(({ name }) => ({ key: name, label: name }));

    if (namespaces.length <= 1) return null;

    // Create default namespace selection
    namespaces.unshift({
      key: ALL_NAMESPACE_WILDCARD,
      label: 'All (*)',
    });

    return namespaces;
  }
}
