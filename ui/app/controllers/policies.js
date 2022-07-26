import { action } from '@ember/object';
import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';

const ALL_NAMESPACE_WILDCARD = '*';

export default class PoliciesController extends Controller {
  queryParams = [{ qpNamespace: 'namespace' }];

  @tracked
  qpNamespace = ALL_NAMESPACE_WILDCARD;

  @action
  setNamespace(namespace) {
    this.qpNamespace = namespace;
  }
}
