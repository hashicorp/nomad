// @ts-check

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class ClusterVizFlyoutComponent extends Component {
  @service cluster;

  @tracked elementEstablished = false;
  @action establishElementDimensions(element) {
    setTimeout(() => {
      this.elementEstablished = true;
    }, 300);
  }

  @action clearElementDimensions(element) {
    this.elementEstablished = false;
  }
}
