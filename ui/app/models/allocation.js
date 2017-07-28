import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';

const { computed } = Ember;

export default Model.extend({
  job: belongsTo('job'),
  node: belongsTo('node'),
  name: attr('string'),
  taskGroupName: attr('string'),
  resources: fragment('resources'),

  taskGroup: computed('taskGroupName', 'job.taskGroups.[]', function() {
    return this.get('job.taskGroups').findBy('name', this.get('taskGroupName'));
  }),

  states: fragmentArray('task-state'),
});
