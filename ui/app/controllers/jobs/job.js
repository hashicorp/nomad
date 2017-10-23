import Ember from 'ember';

const { Controller, computed } = Ember;

export default Controller.extend({
  breadcrumbs: computed('model.{name,id}', function() {
    return [
      { label: 'Jobs', args: ['jobs'] },
      {
        label: this.get('model.name'),
        args: ['jobs.job', this.get('model')],
      },
    ];
  }),
});
