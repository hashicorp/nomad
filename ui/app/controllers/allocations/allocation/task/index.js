import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import { alias } from '@ember/object/computed';
import { task } from 'ember-concurrency';

export default Controller.extend({
  otherTasks: computed('model.task.taskGroup.tasks.@each.name', function() {
    const taskName = this.model.task.name;
    return this.model.task.taskGroup.tasks.rejectBy('name', taskName);
  }),

  prestartTasks: computed('otherTasks.@each.lifecycle', function() {
    return this.otherTasks.mapBy('lifecycle').compact();
  }),

  network: alias('model.resources.networks.firstObject'),
  ports: computed('network.reservedPorts.[]', 'network.dynamicPorts.[]', function() {
    return (this.get('network.reservedPorts') || [])
      .map(port => ({
        name: port.Label,
        port: port.Value,
        isDynamic: false,
      }))
      .concat(
        (this.get('network.dynamicPorts') || []).map(port => ({
          name: port.Label,
          port: port.Value,
          isDynamic: true,
        }))
      )
      .sortBy('name');
  }),

  error: overridable(() => {
    // { title, description }
    return null;
  }),

  onDismiss() {
    this.set('error', null);
  },

  restartTask: task(function*() {
    try {
      yield this.model.restart();
    } catch (err) {
      this.set('error', {
        title: 'Could Not Restart Task',
        description: 'Your ACL token does not grant allocation lifecycle permissions.',
      });
    }
  }),
});
