import Ember from 'ember';

const { Route } = Ember;

export default Route.extend({
  actions: {
    didTransition() {
      window.scrollTo(0, 0);
    },
  },
});
