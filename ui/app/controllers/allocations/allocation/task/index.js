import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import { task } from 'ember-concurrency';
import classic from 'ember-classic-decorator';

@classic
export default class IndexController extends Controller {
  @computed('model.task.taskGroup.tasks.@each.name')
  get otherTaskStates() {
    const taskName = this.model.task.name;
    return this.model.allocation.states.rejectBy('name', taskName);
  }

  @computed('otherTaskStates.@each.lifecycle')
  get prestartTaskStates() {
    return this.otherTaskStates.filterBy('task.lifecycle');
  }

  @overridable(() => {
    // { title, description }
    return null;
  })
  error;

  onDismiss() {
    this.set('error', null);
  }

  @task(function*() {
    try {
      yield this.model.restart();
    } catch (err) {
      this.set('error', {
        title: 'Could Not Restart Task',
        description: 'Your ACL token does not grant allocation lifecycle permissions.',
      });
    }
  })
  restartTask;
}
