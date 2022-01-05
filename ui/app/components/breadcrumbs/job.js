import { assert } from '@ember/debug';
import { action } from '@ember/object';
import Component from '@glimmer/component';

export default class BreadcrumbsJob extends Component {
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
