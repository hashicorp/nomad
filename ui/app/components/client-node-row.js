/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { lazyClick } from '../helpers/lazy-click';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithVisibilityDetection from 'nomad-ui/mixins/with-component-visibility-detection';
import { computed } from '@ember/object';
import { classNames, tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('tr')
@classNames('client-node-row', 'is-interactive')
export default class ClientNodeRow extends Component.extend(
  WithVisibilityDetection
) {
  @service store;

  node = null;

  onClick() {}

  click(event) {
    lazyClick([this.onClick, event]);
  }

  didReceiveAttrs() {
    super.didReceiveAttrs();
    // Reload the node in order to get detail information
    const node = this.node;
    if (node) {
      node.reload().then(() => {
        this.watch.perform(node, 100);
      });
    }
  }

  visibilityHandler() {
    if (document.hidden) {
      this.watch.cancelAll();
    } else {
      const node = this.node;
      if (node) {
        this.watch.perform(node, 100);
      }
    }
  }

  willDestroy() {
    this.watch.cancelAll();
    super.willDestroy(...arguments);
  }

  @watchRelationship('allocations') watch;

  @computed('node.compositeStatus')
  get compositeStatusClass() {
    let compositeStatus = this.get('node.compositeStatus');

    if (compositeStatus === 'draining') {
      return 'status-text is-info';
    } else if (compositeStatus === 'ineligible') {
      return 'status-text is-warning';
    } else if (compositeStatus === 'down') {
      return 'status-text is-danger';
    } else {
      return '';
    }
  }
}
