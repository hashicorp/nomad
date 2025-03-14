/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Service from '@ember/service';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

export default class HistoryService extends Service {
  @service router;

  constructor() {
    super(...arguments);
    console.log('history.constructor');
    this.router.on('routeDidChange', this, this.addRoute);
  }

  validEntities = [
    'Job',
    'TaskGroup',
    'Allocation',
    'TaskState', // task
    'Node', // client
    'Agent', // server
    'SentinelPolicy',
    'VariableModel', // variable
  ];

  /**
   * @typedef {import('@ember/routing/route-info').RouteInfoWithAttributes} RouteInfoWithAttributes
   */

  /**
   * @typedef {Object} Recent
   * @property {string} entityType - The type of the entity (e.g., Job, TaskGroup, Node).
   * @property {string} id - The unique identifier of the entity.
   * @property {string} label - The displayed name of the entity.
   * @property {Array<number>} views - An array of timestamps representing view times.
   * @property {Object | null} transitionFrom
  //  * TODO: transitionFrom should actually be an array, or else id'be be mostRecentTransitionFrom.
   * @property {string} transitionFrom.entityType - The type of the entity from which the user transitioned.
   * @property {string} transitionFrom.id - The unique identifier of the entity from which the user transitioned.
   * @property {number} lastVisit - The timestamp of the last visit.
   * 
   */

  /**
   *
   * @param {import("@ember/routing/transition").default} transition
   */
  addRoute(transition) {
    console.log('history.addRoute', transition.to);
    const fromRoute = /** @type {RouteInfoWithAttributes | null} */ (
      transition.from
    );
    const toRoute = /** @type {RouteInfoWithAttributes | null} */ (
      transition.to
    );

    const fromRouteEntityType =
      fromRoute?.attributes?.constructor?.name || 'N/A';
    const toRouteEntityType = toRoute?.attributes?.constructor?.name || 'N/A';

    console.log(
      `Transitioning from ${fromRouteEntityType} to ${toRouteEntityType}`
    );
    console.log('validEntities', this.validEntities);

    if (!this.validEntities.includes(toRouteEntityType)) {
      console.log(`Ignoring route ${toRouteEntityType}`);
      return;
    }

    const toRouteIdentifier = toRoute.attributes.id || toRoute.attributes.name;
    const toRouteLabel = toRoute.attributes.name || toRoute.attributes.id;

    // let recents = this.recents || [];
    const recents = /** @type {Array<Recent>} */ (this.recents || []);
    // const recent = recents.find(r => r.entityType === toRouteEntityType && (r.id ? r.id === toRouteIdentifier : r.name === toRouteIdentifier));
    const recent = recents.find(
      (r) =>
        r.entityType === toRouteEntityType &&
        (r.id ? r.id === toRouteIdentifier : r.label === toRouteIdentifier)
    );
    console.log(
      'recents, recent',
      recents,
      toRouteEntityType,
      toRouteIdentifier
    );

    const now = Date.now();

    console.log('=== recent found?', recent);
    if (recent) {
      recent.views.push(now);
      console.log('recent UPDATED');
    } else {
      /** @type {Recent} */
      let recentObjectToAdd = {
        entityType: toRouteEntityType,
        id: toRouteIdentifier,
        label: toRouteLabel,
        views: [now],
        lastVisit: now,
        transitionFrom:
          fromRoute?.attributes.id && fromRouteEntityType
            ? { entityType: fromRouteEntityType, id: fromRoute?.attributes.id }
            : null,
      };
      recents.push(recentObjectToAdd);
      console.log('recent ADDED');
    }
    this.recents = [...recents];
    // this.recents = recents;
    // this.set('recents', recents);
    set(this, 'recents', recents);
    console.log('recents', this.recents);
    // recents.push({
    //   entityType: toRouteEntityType,
    //   id: toRoute.attributes.id,
    //   lastVisit: new Date(),
    //   transitionFrom: [{fromRouteEntityType}],
    // });
    // }
  }

  get recommendations() {
    // order recents by views length
    let recents = this.recents;
    recents.forEach((r) => (r.reason = null)); // TODO: temp
    console.log('CURRENT ROUTE CONST NAME', this.router.currentRoute, recents);
    let routeRelevantRecents = recents.filter((r) => {
      // check to see if currentROute matches transitionFrom.entityType and id
      return (
        r.transitionFrom?.entityType ===
          this.router.currentRoute.attributes.constructor.name &&
        r.transitionFrom?.id === this.router.currentRoute.attributes.id
      );
    });
    console.log('RRR', routeRelevantRecents);
    routeRelevantRecents.forEach(
      (r) => (r.reason = 'Recently viewed after your current page')
    );
    let sortedRecents = recents.sort((a, b) => b.views.length - a.views.length);
    return [...routeRelevantRecents, ...sortedRecents].slice(0, 4);
    return sortedRecents.slice(0, 4);
    // TODO: do some smooth things like make lastVisit matter, and make sure it's not the current route
    // and even some advanced stuff like checking transitionFrom to see if it's a related entity
  }

  @localStorageProperty('nomadRecentUIVisits', []) recents;

  prune() {
    console.log('history.prune');
  }
}
