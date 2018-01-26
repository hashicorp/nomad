import Route from '@ember/routing/route';
import EmberError from '@ember/error';

export default Route.extend({
  model() {
    const err = new EmberError('Page not found');
    err.code = '404';
    this.controllerFor('application').set('error', err);
  },
});
