/* eslint-disable ember/no-controller-access-in-routes */
import Route from '@ember/routing/route';
import EmberError from '@ember/error';

export default class NotFoundRoute extends Route {
  model() {
    const err = new EmberError('Page not found');
    err.code = '404';
    this.controllerFor('application').set('error', err);
  }
}
