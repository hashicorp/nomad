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
  @service router;

  @localStorageProperty('nomadShouldWrapCode', false) wordWrap;
  @localStorageProperty('nomadLiveUpdateJobsIndex', true) liveUpdateJobsIndex;
  // @localStorageProperty('nomadDefaultNamespace') userDefaultNamespace;
  // @alias('system.userDefaultNamespace') userDefaultNamespace;
  // @localStorageProperty('nomadDefaultNodePool') defaultNodePool;

  @alias('model.namespaces') namespaces;
  @alias('model.nodePools') nodePools;

  @tracked namespaceFilter = '';
  @tracked nodePoolFilter = '';

  @computed('namespaces.[]', 'namespaceFilter')
  get filteredNamespaces() {
    return this.namespaces.filter((ns) =>
      ns.name.includes(this.namespaceFilter)
    );
  }

  @computed('namespaces', 'nodePoolFilter', 'nodePools.[]')
  get filteredNodepools() {
    console.log('--filtNodePools', this.nodePools, this.namespaces);
    return this.nodePools.filter((np) => np.name.includes(this.nodePoolFilter));
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

  /**
   * @type {Array<string>} defaultNodePool
   */
  get nodepoolDefaults() {
    return this.globalDefaults.nodePool;
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
    let userDefaultNamespaces = this.system.userDefaultNamespace
      ? this.system.userDefaultNamespace.split(',')
      : [];

    let agentDefaultNamespaces = this.system.agentDefaults?.Namespace
      ? this.system.agentDefaults.Namespace.split(',')
      : [];

    if (userDefaultNamespaces.length) {
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

  @computed(
    'filteredNodepools.@each.checked',
    'nodepoolDefaults',
    'nodepoolFilter',
    'nodepool.[]'
  )
  get nodepoolOptions() {
    return this.filteredNodepools.map((np) => ({
      label: np.name,
      value: np.name,
      checked: this.nodepoolDefaults.includes(np.name),
    }));
  }

  get nodepoolDropdownLabel() {
    let userDefaultNodePools = this.system.userDefaultNodePool
      ? this.system.userDefaultNodePool.split(',')
      : [];

    let agentDefaultNodePools = this.system.agentDefaults?.NodePool
      ? this.system.agentDefaults.NodePool.split(',')
      : [];

    if (userDefaultNodePools.length) {
      return `${userDefaultNodePools.length} default node pool${
        userDefaultNodePools.length > 1 ? 's' : ''
      } (via localStorage)`;
    } else if (agentDefaultNodePools.length) {
      return `${agentDefaultNodePools.length} default node pool${
        agentDefaultNodePools.length > 1 ? 's' : ''
      } (via agent config)`;
    } else {
      return 'No default node pool set';
    }
  }

  // @action setAllNamespacesAsDefault() {
  //   this.namespaceOptions.forEach((ns) => set(ns, 'checked', true));
  //   let checkedNamespaces = this.namespaceOptions.mapBy('label');
  //   set(this, 'system.userDefaultNamespace', checkedNamespaces.join(', '));
  //   set(this, 'model.defaults.namespace', checkedNamespaces); // explicitly modify the model to trigger a refresh
  //   this.notifications.add({
  //     title: 'Default Namespace Updated',
  //     message: 'All namespaces are now default',
  //     color: 'success',
  //   });
  // }

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

  @action clearUserDefaultNamespaces() {
    set(this, 'system.userDefaultNamespace', null);
    this.notifications.add({
      title: 'Default Namespaces Cleared',
      message: this.system.agentDefaults?.Namespace
        ? `Namespaces ${this.system.agentDefaults.Namespace} are now default via agent`
        : 'No default namespaces set',
      color: 'success',
    });
  }

  @action setDefaultNodePool(np, evt) {
    np.checked = evt.target.checked;
    let checkedNodePools = this.nodepoolOptions
      .filterBy('checked')
      .mapBy('label');
    if (!checkedNodePools.length) {
      set(this, 'system.userDefaultNodePool', null);
      // this.model.defaults.nodePool = this.system.agentDefaults.NodePool.split(',');
    } else {
      set(this, 'system.userDefaultNodePool', checkedNodePools.join(', '));
      set(this, 'model.defaults.nodePool', checkedNodePools); // explicitly modify the model to trigger a refresh
    }
    this.notifications.add({
      title: 'Default Node Pool Updated',
      message: np.checked
        ? `Node Pool ${np.label} is now default`
        : `Node Pool ${np.label} is no longer default`,
      color: 'success',
    });
  }

  @action clearUserDefaultNodePools() {
    set(this, 'system.userDefaultNodePool', null);
    this.notifications.add({
      title: 'Default Node Pools Cleared',
      message: this.system.agentDefaults?.NodePool
        ? `Node Pools ${this.system.agentDefaults.NodePool} are now default via agent`
        : 'No default node pools set',
      color: 'success',
    });
  }

  get sortedRegions() {
    console.log('sortreg', this.system.regions);
    return this.system.regions.toArray().sort();
  }

  @action setDefaultRegion(region) {
    this.system.userDefaultRegion = region;

    // When you set region via the region switcher in the header, it ships you over to the jobs route with a ?region= queryParam.
    // This maintains that convention and un-sets it if you set your default as the authoritative region.
    this.router.transitionTo('settings.user-settings', {
      queryParams: { region },
    });

    this.notifications.add({
      title: 'Default Region Updated',
      message: `Region ${region} is now default`,
      color: 'success',
    });
  }

  // You may be asking: why isn't there a clearDefaultRegion() action like there is for namespaces?
  // Your localStorage settings get an activeRegion/default region every time you change regions via the header switcher,
  // but also when you load the application route, so at best you'd have "no default" until you refresh the page.
  // If we ever decide to get rid of "save your active region for when you load the app in the future" as a convention,
  // then we can add a clearDefaultRegion() action.
}
