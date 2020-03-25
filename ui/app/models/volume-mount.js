import { computed } from '@ember/object';
import { alias, equal } from '@ember/object/computed';
import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  task: fragmentOwner(),

  volume: attr('string'),

  volumeDeclaration: computed('task.taskGroup.volumes.@each.name', function() {
    return this.task.taskGroup.volumes.findBy('name', this.volume);
  }),

  isCSI: equal('volumeDeclaration.type', 'csi'),
  source: alias('volumeDeclaration.source'),

  // Since CSI volumes are namespaced, the link intent of a volume mount will
  // be to the CSI volume with a namespace that matches this task's job's namespace.
  namespace: alias('task.taskGroup.job.namespace'),

  destination: attr('string'),
  propagationMode: attr('string'),
  readOnly: attr('boolean'),
});
