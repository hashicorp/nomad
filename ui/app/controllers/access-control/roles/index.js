/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';

export default class AccessControlRolesIndexController extends Controller {
  @service router;
  @service notifications;
  @service can;

  get columns() {
    const defaultColumns = [
      {
        key: 'name',
        label: 'Name',
        isSortable: true,
      },
      {
        key: 'description',
        label: 'Description',
      },
    ];

    const policiesColumn = {
      key: 'policies',
      label: 'Policies',
    };

    const tokensColumn = {
      key: 'tokens',
      label: 'Tokens',
      isSortable: true,
    };

    const deleteColumn = {
      key: 'delete',
      label: 'Delete',
    };

    return [
      ...defaultColumns,
      ...(this.can.can('list token') ? [tokensColumn] : []),
      ...(this.can.can('list policy') ? [policiesColumn] : []),
      ...(this.can.can('destroy role') ? [deleteColumn] : []),
    ];
  }

  get roles() {
    return this.model.roles.map((role) => {
      role.tokens = (this.model.tokens || []).filter((token) => {
        return token.roles.includes(role);
      });
      return role;
    });
  }

  @action openRole(role) {
    this.router.transitionTo('access-control.roles.role', role.id);
  }

  @action goToNewRole() {
    this.router.transitionTo('access-control.roles.new');
  }

  @task(function* (role) {
    try {
      yield role.deleteRecord();
      yield role.save();
      this.notifications.add({
        title: `Role ${role.name} successfully deleted`,
        color: 'success',
      });
    } catch (err) {
      this.error = {
        title: 'Error deleting role',
        description: err,
      };

      throw err;
    }
  })
  deleteRole;
}
