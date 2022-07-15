import { assert } from '@ember/debug';
import { action } from '@ember/object';
import BreadcrumbsTemplate from './default';

export default class BreadcrumbsJob extends BreadcrumbsTemplate {
  get job() {
    return this.args.crumb.job;
  }

  @action
  onError(err) {
    assert(`Error:  ${err.message}`);
  }

  @action
  fetchParent() {
    const hasParent = !!this.job.belongsTo('parent').id();
    if (hasParent) {
      return this.job.get('parent');
    }
  }
}
