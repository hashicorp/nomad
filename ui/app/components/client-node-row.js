/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { lazyClick } from '../helpers/lazy-click';
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
      node.reload();
    }
  }

  @computed('node.status')
  get nodeStatusColor() {
    let status = this.get('node.status');
    if (status === 'disconnected') {
      return 'warning';
    } else if (status === 'down') {
      return 'critical';
    } else if (status === 'ready') {
      return 'success';
    } else if (status === 'initializing') {
      return 'neutral';
    } else {
      return 'neutral';
    }
  }
  @computed('node.status')
  get nodeStatusIcon() {
    let status = this.get('node.status');
    if (status === 'disconnected') {
      return 'skip';
    } else if (status === 'down') {
      return 'x-circle';
    } else if (status === 'ready') {
      return 'check-circle';
    } else if (status === 'initializing') {
      return 'entry-point';
    } else {
      return '';
    }
  }
}
