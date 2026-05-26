/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { service } from '@ember/service';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';

export default class AccessControlRolesIndexController extends Controller {
  @service router;
  @service notifications;
  @service abilities;

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
      ...(this.abilities.can('list token') ? [tokensColumn] : []),
      ...(this.abilities.can('list policy') ? [policiesColumn] : []),
      ...(this.abilities.can('destroy role') ? [deleteColumn] : []),
    ];
  }

  get roles() {
    return this.model.roles.map((role) => {
      const tokens = (this.model.tokens || []).filter((token) => {
        return token.roles.includes(role);
      });

      return {
        id: role.id,
        name: role.name,
        description: role.description,
        policyNames: role.policyNames,
        tokens,
        record: role,
      };
    });
  }

  @action openRole(role) {
    this.router.transitionTo('administration.roles.role', role.id);
  }

  @action goToNewRole() {
    this.router.transitionTo('administration.roles.new');
  }

  @task(function* (role) {
    try {
      const record = role.record || role;

      yield record.deleteRecord();
      yield record.save();
      this.notifications.add({
        title: `Role ${record.name} successfully deleted`,
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
