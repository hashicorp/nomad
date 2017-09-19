import Ember from 'ember';

const { Route } = Ember;

export default Route.extend({
  redirect() {
    this.transitionTo('jobs');
  },
});
