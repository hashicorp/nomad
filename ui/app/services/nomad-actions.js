/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Guess who just found out that "actions" is a reserved name in Ember?
// Signed, the person who just renamed this NomadActions.

// @ts-check
import Service from '@ember/service';
import { inject as service } from '@ember/service';

export default class NomadActionsService extends Service {
  @service can;

  // Note: future Actions Governance work (https://github.com/hashicorp/nomad/issues/18800)
  // will require this to be a computed property that depends on the current user's permissions.
  // For now, we simply check alloc exec privileges.
  get hasActionPermissions() {
    return this.can.can('exec allocation');
  }
}
