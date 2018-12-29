import Route from '@ember/routing/route';

export default Route.extend({
  redirect() {
    this.transitionTo('jobs');
  },
});
