import Ember from 'ember';

const { Route } = Ember;

export default Route.extend({
  model({ name }) {
    return this.modelFor('jobs.job').get('taskGroups').findBy('name', name);
  },
});
