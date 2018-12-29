import Ember from 'ember';

const { Route, Error: EmberError } = Ember;

export default Route.extend({
  model() {
    const err = new EmberError('Page not found');
    err.code = '404';
    this.controllerFor('application').set('error', err);
  },
});
