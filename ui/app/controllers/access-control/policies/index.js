/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';

export default class AccessControlPoliciesIndexController extends Controller {
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
      ...(this.can.can('destroy policy') ? [deleteColumn] : []),
    ];
  }

  get policies() {
    return this.model.policies.map((policy) => {
      policy.tokens = (this.model.tokens || []).filter((token) => {
        return token.policies.includes(policy);
      });
      return policy;
    });
  }

  @action openPolicy(policy) {
    this.router.transitionTo('access-control.policies.policy', policy.name);
  }

  @action goToNewPolicy() {
    this.router.transitionTo('access-control.policies.new');
  }

  @task(function* (policy) {
    try {
      yield policy.deleteRecord();
      yield policy.save();
      this.notifications.add({
        title: `Policy ${policy.name} successfully deleted`,
        color: 'success',
      });
    } catch (err) {
      this.error = {
        title: 'Error deleting policy',
        description: err,
      };

      throw err;
    }
  })
  deletePolicy;
}
