/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { service } from '@ember/service';
import { action } from '@ember/object';
import { task } from 'ember-concurrency';

export default class AccessControlPoliciesIndexController extends Controller {
  @service store;
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
      ...(this.abilities.can('destroy policy') ? [deleteColumn] : []),
    ];
  }

  get policies() {
    return this.model.policies.map((policy) => {
      const tokens = (this.model.tokens || []).filter((token) => {
        return token.policies.includes(policy);
      });

      return {
        id: policy.id,
        name: policy.name,
        description: policy.description,
        rules: policy.rules,
        rulesJSON: policy.rulesJSON,
        tokens,
        record: policy,
      };
    });
  }

  @action openPolicy(policy) {
    this.router.transitionTo('administration.policies.policy', policy.name);
  }

  @action goToNewPolicy() {
    this.router.transitionTo('administration.policies.new');
  }

  @task(function* (policy) {
    try {
      const record = policy.record || policy;

      yield record.deleteRecord();
      yield record.save();

      // Cleanup: Remove references from roles and tokens
      this.store.peekAll('role').forEach((role) => {
        role.policies.removeObject(record);
      });
      this.store.peekAll('token').forEach((token) => {
        token.policies.removeObject(record);
      });
      if (this.store.peekRecord('policy', record.id)) {
        this.store.unloadRecord(record);
      }

      this.notifications.add({
        title: `Policy ${record.name} successfully deleted`,
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
