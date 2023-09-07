/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class AccessControlRolesIndexController extends Controller {
  @service router;
  get roles() {
    return this.model.roles.map((role) => {
      role.tokens = (this.model.tokens || []).filter((token) => {
        return token.roles.includes(role);
      });
      return role;
    });
  }

  @action openRole(role) {
    this.router.transitionTo('access-control.roles.role', role.name);
  }
}
