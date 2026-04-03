/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { compare } from '@ember/utils';
import { get } from '@ember/object';
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class AllocationServiceSidebarComponent extends Component {
  @service store;
  @service system;

  get isSideBarOpen() {
    return !!this.args.service;
  }
  keyCommands = [
    {
      label: 'Close Service Sidebar',
      pattern: ['Escape'],
      action: () => this.args.fns.closeSidebar(),
    },
  ];

  get service() {
    return this.store.query('service-fragment', { refID: this.args.serviceID });
  }

  get address() {
    const port = this.args.allocation?.allocatedResources?.ports?.find(
      (item) => get(item, 'label') === this.args.service.portLabel
    );
    if (port) {
      return `${port.hostIp}:${port.value}`;
    } else {
      return null;
    }
  }

  get aggregateStatus() {
    if (this.args.allocation?.clientStatus !== 'running') return 'Unknown';
    return this.checks.some((check) => check.Status === 'failure')
      ? 'Unhealthy'
      : 'Healthy';
  }

  get consulRedirectLink() {
    return this.system.agent.get('config')?.UI?.Consul?.BaseUIURL;
  }

  get checks() {
    if (!this.args.service || !this.args.allocation) return [];
    let allocID = this.args.allocation.id;
    // Our UI checks run every 2 seconds; but a check itself may only update every, say, minute.
    // Therefore, we'll have duplicate checks in a service's healthChecks array.
    // Only get the most recent check for each check.
    return [
      ...[
        ...(this.args.service.healthChecks || []).filter(
          (item) => get(item, 'Alloc') === allocID
        ),
      ]
        .sort((a, b) => compare(get(a, 'Timestamp'), get(b, 'Timestamp')))
        .reverse()
        .reduce(
          ([uniqArr, itemsSet, getterFn], item) => {
            const val = getterFn(item);
            if (!itemsSet.has(val)) {
              itemsSet.add(val);
              uniqArr.push(item);
            }
            return [uniqArr, itemsSet, getterFn];
          },
          [[], new Set(), (item) => get(item, 'Check')]
        )[0],
    ].sort((a, b) => compare(get(a, 'Check'), get(b, 'Check')));
  }
}
