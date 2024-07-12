/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';
import { set, action } from '@ember/object';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';

export default class SettingsUserSettingsController extends Controller {
  @service notifications;
  @service system;

  @localStorageProperty('nomadShouldWrapCode', false) wordWrap;
  @localStorageProperty('nomadLiveUpdateJobsIndex', true) liveUpdateJobsIndex;
  // @localStorageProperty('nomadDefaultNamespace') userDefaultNamespace;
  @alias('system.userDefaultNamespace') userDefaultNamespace;
  // @localStorageProperty('nomadDefaultNodePool') defaultNodePool;

  @alias('model.namespaces') namespaces;
  @alias('model.nodePools') nodePools;

  @tracked namespaceFilter = '';
  get filteredNamespaces() {
    return this.namespaces.filter((ns) =>
      ns.name.includes(this.namespaceFilter)
    );
  }

  /**
   * @type {import ('../../services/system').RenderedDefaults} RenderedDefaults
   */
  get globalDefaults() {
    return this.model.defaults;
  }

  /**
   * @type {Array<string>} defaultNamespace
   */
  get namespaceDefaults() {
    return this.globalDefaults.namespace;
  }

  @computed(
    'filteredNamespaces.@each.checked',
    'namespaceDefaults',
    'namespaceFilter',
    'namespaces.[]'
  )
  get namespaceOptions() {
    return this.filteredNamespaces.map((ns) => ({
      label: ns.name,
      value: ns.name,
      checked: this.namespaceDefaults.includes(ns.name),
    }));
  }

  get namespaceDropdownLabel() {
    let userDefaultNamespaces = this.userDefaultNamespace
      ? this.userDefaultNamespace.split(',')
      : [];
    let agentDefaultNamespaces = this.system.agentDefaults?.Namespace
      ? this.system.agentDefaults.Namespace.split(',')
      : [];
    if (this.userDefaultNamespace) {
      return `${userDefaultNamespaces.length} default namespace${
        userDefaultNamespaces.length > 1 ? 's' : ''
      } (via localStorage)`;
    } else if (agentDefaultNamespaces.length) {
      return `${agentDefaultNamespaces.length} default namespace${
        agentDefaultNamespaces.length > 1 ? 's' : ''
      } (via agent config)`;
    } else {
      return 'No default namespace set';
    }
  }

  @action setAllNamespacesAsDefault() {
    this.namespaceOptions.forEach((ns) => set(ns, 'checked', true));
    let checkedNamespaces = this.namespaceOptions.mapBy('label');
    set(this, 'system.userDefaultNamespace', checkedNamespaces.join(', '));
    set(this, 'model.defaults.namespace', checkedNamespaces); // explicitly modify the model to trigger a refresh
    this.notifications.add({
      title: 'Default Namespace Updated',
      message: 'All namespaces are now default',
      color: 'success',
    });
  }

  @action setDefaultNamespace(ns, evt) {
    ns.checked = evt.target.checked;
    let checkedNamespaces = this.namespaceOptions
      .filterBy('checked')
      .mapBy('label');
    if (!checkedNamespaces.length) {
      set(this, 'system.userDefaultNamespace', null);
      // this.model.defaults.namespace = this.system.agentDefaults.Namespace.split(',');
    } else {
      set(this, 'system.userDefaultNamespace', checkedNamespaces.join(', '));
      set(this, 'model.defaults.namespace', checkedNamespaces); // explicitly modify the model to trigger a refresh
    }
    this.notifications.add({
      title: 'Default Namespace Updated',
      message: ns.checked
        ? `Namespace ${ns.label} is now default`
        : `Namespace ${ns.label} is no longer default`,
      color: 'success',
    });
  }
}
