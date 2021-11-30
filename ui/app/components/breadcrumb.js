import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class Breadcrumb extends Component {
  @service bucket;

  @action register() {
    this.bucket.registerBreadcrumb(this);
  }

  @action deregister() {
    this.bucket.deregisterBreadcrumb(this);
  }

  constructor() {
    super(...arguments);
    this.register();
  }

  willDestroy() {
    super.willDestroy();
    this.deregister();
  }
}
