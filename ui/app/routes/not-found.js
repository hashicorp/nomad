/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-controller-access-in-routes */
import Route from '@ember/routing/route';

export default class NotFoundRoute extends Route {
  model() {
    const err = new Error('Page not found');
    err.code = '404';
    this.controllerFor('application').set('error', err);
  }
}
