/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class ProfileNavbarItemComponent extends Component {
  @service token;
  @service router;
  @service store;

  @action
  signOut() {
    this.token.setProperties({
      secret: undefined,
    });

    // Clear out all data to ensure only data the anonymous token is privileged to see is shown
    this.store.unloadAll();
    this.token.reset();
    this.router.transitionTo('jobs.index');
  }
}
