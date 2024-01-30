/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

const ALL_NAMESPACE_WILDCARD = '*';

export default class JobsIndexController extends Controller {
  @service router;
  @service system;
  @service watchList; // TODO: temp

  queryParams = [
    'cursorAt',
    'pageSize',
    // 'status',
    { qpNamespace: 'namespace' },
    // 'type',
    // 'searchTerm',
  ];

  isForbidden = false;

  get tableColumns() {
    return [
      'name',
      this.system.shouldShowNamespaces ? 'namespace' : null,
      this.system.shouldShowNodepools ? 'node pools' : null, // TODO: implement on system service
      'status',
      'type',
      'priority',
      'running allocations',
    ]
      .filter((c) => !!c)
      .map((c) => {
        return {
          label: c.charAt(0).toUpperCase() + c.slice(1),
          width: c === 'running allocations' ? '200px' : undefined,
        };
      });
  }

  @tracked jobs = [];

  @action
  gotoJob(job) {
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  }

  // #region pagination
  @tracked cursorAt;
  @tracked nextToken; // route sets this when new data is fetched
  @tracked previousTokens = [];

  /**
   *
   * @param {"prev"|"next"} page
   */
  @action handlePageChange(page, event, c) {
    console.log('hPC', page, event, c);
    // event.preventDefault();
    if (page === 'prev') {
      console.log('prev page');
      this.cursorAt = this.previousTokens.pop();
      this.previousTokens = [...this.previousTokens];
    } else if (page === 'next') {
      console.log('next page', this.nextToken);
      this.previousTokens = [...this.previousTokens, this.cursorAt];
      this.cursorAt = this.nextToken;
    }
  }
  // #endregion pagination
}
