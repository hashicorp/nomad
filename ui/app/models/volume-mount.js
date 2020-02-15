import { computed } from '@ember/object';
import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  task: fragmentOwner(),

  volume: attr('string'),
  source: computed('volume', 'task.taskGroup.volumes.@each.{name,source}', function() {
    return this.task.taskGroup.volumes.findBy('name', this.volume).source;
  }),

  destination: attr('string'),
  propagationMode: attr('string'),
  readOnly: attr('boolean'),
});
