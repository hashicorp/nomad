/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class SettingsUserSettingsController extends Controller {
  @service notifications;

  @localStorageProperty('nomadShouldWrapCode', false) wordWrap;
  @localStorageProperty('nomadLiveUpdateJobsIndex', true) liveUpdateJobsIndex;
  @localStorageProperty('nomadDefaultNamespace') defaultNamespace;
  @localStorageProperty('nomadDefaultNodePool') defaultNodePool;

  @alias('model.namespaces') namespaces;
  @alias('model.nodePools') nodePools;

  @tracked namespaceFilter = '';
  get filteredNamespaces() {
    return this.namespaces.filter((ns) =>
      ns.name.includes(this.namespaceFilter)
    );
  }

  @action setDefaultNamespace(ns) {
    console.log('setDefaultNamespace', ns);
    this.defaultNamespace = ns;

    this.notifications.add({
      title: 'Default Namespace Updated',
      message: ns
        ? `Default namespace is now ${ns}`
        : 'Default namespace un-set',
      color: 'success',
    });
  }
}
