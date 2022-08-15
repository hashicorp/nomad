import { assert } from '@ember/debug';
import { action } from '@ember/object';
import Component from '@glimmer/component';

export default class BreadcrumbsJob extends Component {
  get job() {
    return this.args.crumb.job;
  }

  get hasParent() {
    return !!this.job.belongsTo('parent').id();
  }

  @action
  onError(err) {
    assert(`Error:  ${err.message}`);
  }

  @action
  fetchParent() {
    if (this.hasParent) {
      return this.job.get('parent');
    }
  }
}
