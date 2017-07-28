import Ember from 'ember';

const { Controller, computed, inject } = Ember;

export default Controller.extend({
  jobController: inject.controller('jobs.job'),

  breadcrumbs: computed('jobController.breadcrumbs.[]', 'model.{name}', function() {
    return this.get('jobController.breadcrumbs').concat([
      { label: this.get('model.name'), args: ['jobs.job.task-group', this.get('model.name')] },
    ]);
  }),
});
