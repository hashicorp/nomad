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
  // @service store;
  @service system;

  queryParams = [
    'nextToken',
    'pageSize',
    'foo',
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
      'summary',
    ]
      .filter((c) => !!c)
      .map((c) => {
        return {
          label: c.charAt(0).toUpperCase() + c.slice(1),
          width: c === 'summary' ? '200px' : undefined,
        };
      });
  }

  get jobs() {
    return this.model.jobs;
  }

  @action
  gotoJob(job) {
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  }

  // #region pagination
  // @action
  // onNext(nextToken) {
  //   this.previousTokens = [...this.previousTokens, this.nextToken];
  //   this.nextToken = nextToken;
  // }

  // get getPrevToken() {
  //   return "beep";
  // }
  // get getNextToken() {
  //   return "boop";
  // }

  @tracked initialNextToken;
  @tracked nextToken;
  @tracked previousTokens = [null];

  @action someFunc(a, b, c) {
    console.log('someFunc called', a, b, c);
  }

  /**
   *
   * @param {"prev"|"next"} page
   */
  @action handlePageChange(page, event, c) {
    console.log('hPC', page, event, c);
    // event.preventDefault();
    if (page === 'prev') {
      console.log('prev page');
      this.nextToken = this.previousTokens.pop();
      this.previousTokens = [...this.previousTokens];
    } else if (page === 'next') {
      console.log('next page', this.model.jobs.meta);
      this.previousTokens = [...this.previousTokens, this.nextToken];
      // this.nextToken = "boop";
      // random
      // this.nextToken = Math.random().toString(36).substring(7);
      this.nextToken = this.model.jobs.meta.nextToken;
      // this.foo = "bar";
    }
  }

  // get paginate() {
  //   console.log('paginating');
  //   return (page,b,c) => {
  //     return {
  //       // nextToken: this.nextToken,
  //       nextToken: "boop",
  //     }
  //   }
  // }

  // get demoQueryFunctionCompact() {
  //   return (page,b,c) => {
  //     console.log('demoQueryFunctionCompact', page, b,c);
  //     return {
  //       // demoCurrentToken: page === 'prev' ? this.getPrevToken : this.getNextToken,
  //       // demoExtraParam: 'hello',
  //       nextToken: page === 'prev' ? this.getPrevToken : this.getNextToken,
  //     };
  //   };
  // }
  // #endregion pagination
}
