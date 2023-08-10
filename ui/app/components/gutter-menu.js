/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { computed } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class GutterMenu extends Component {
  @service system;
  @service router;
  @service keyboard;

  @computed('system.namespaces.@each.name')
  get sortedNamespaces() {
    const namespaces = this.get('system.namespaces').toArray() || [];

    return namespaces.sort((a, b) => {
      const aName = a.get('name');
      const bName = b.get('name');

      // Make sure the default namespace is always first in the list
      if (aName === 'default') {
        return -1;
      }
      if (bName === 'default') {
        return 1;
      }

      if (aName < bName) {
        return -1;
      }
      if (aName > bName) {
        return 1;
      }

      return 0;
    });
  }

  onHamburgerClick() {}

  // Seemingly redundant, but serves to ensure the action is passed to the keyboard service correctly
  transitionTo(destination) {
    return this.router.transitionTo(destination);
  }
}
