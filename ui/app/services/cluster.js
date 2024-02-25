// @ts-check

import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { singularize } from 'ember-inflector';
import { action } from '@ember/object';

export default class ClusterService extends Service {
  @service router;
  @service store;

  /**
   * @typedef job
   * @property {string} job_name
   */

  /**
   * @typedef client
   * @property {string} node_id
   */

  /**
   * @typedef token
   * @property {string} id
   */

  /**
   * @typedef {job | client | token} entity
   */

  /**
   * @returns {"jobs" | "nodes" | "tokens" }
   */
  get context() {
    if (this.router.currentRouteName.includes('jobs')) {
      return 'jobs';
    } else if (this.router.currentRouteName.includes('clients')) {
      return 'nodes';
    } else if (this.router.currentRouteName.includes('access-control')) {
      return 'tokens';
    }
  }

  /**
   * @returns {entity}
   */
  get entityID() {
    if (
      this.context === 'jobs' &&
      this.router.currentRouteName.includes('jobs.job')
    ) {
      let entityID =
        this.router.currentRoute.params.job_name ||
        this.router.currentRoute.parent.params.job_name;
      console.log('entityID', entityID);
      // Split to model id format
      entityID = JSON.stringify([
        entityID.split('@')[0],
        entityID.split('@')[1],
      ]);
      return entityID;
    }
    if (
      this.context === 'nodes' &&
      this.router.currentRouteName.includes('clients.client')
    ) {
      console.log('oy', this.router.currentRoute);
      return (
        this.router.currentRoute.params.node_id ||
        this.router.currentRoute.parent.params.node_id
      );
    }
  }

  get entity() {
    // if (this.context === "jobs") {
    //   return this.store.peekRecord('job', this.entityID);
    // } else if (this.context === "clients") {
    //   return this.store.peekRecord('client', this.entityID);
    // } else if (this.context === "tokens") {
    //   return this.store.peekRecord('token', this.entityID);
    // }
    // use singularize to get the model name
    if (!this.context || !this.entityID) {
      return;
    }
    console.log(
      'ahoy',
      singularize(this.context),
      this.entityID,
      this.store.peekAll(singularize(this.context))
    );
    return this.store.peekRecord(singularize(this.context), this.entityID);
  }

  /**
   * @returns {string}
   */
  get currentRouteName() {
    return this.router.currentRouteName;
  }

  get data() {
    console.log('=== service data change');
    console.log('updates on peekAll jobs, say');
    if (!this.context) {
      return;
    }
    // return this.store.peekAll(singularize(this.context));
    return this.store.peekAll('job');
  }

  get servers() {
    return this.store.findAll('agent');
  }

  get clients() {
    return this.store.findAll('node');
  }

  @tracked flyoutActive = false;
  @action openFlyout() {
    this.flyoutActive = true;
  }
  @action closeFlyout() {
    this.flyoutActive = false;
  }
}
