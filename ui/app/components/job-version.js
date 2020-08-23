import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

const changeTypes = ['Added', 'Deleted', 'Edited'];

@classic
@classNames('job-version', 'boxed-section')
export default class JobVersion extends Component {
  version = null;
  isOpen = false;

  // Passes through to the job-diff component
  verbose = true;

  @computed('version.diff')
  get changeCount() {
    const diff = this.get('version.diff');
    const taskGroups = diff.TaskGroups || [];

    if (!diff) {
      return 0;
    }

    return (
      fieldChanges(diff) +
      taskGroups.reduce(arrayOfFieldChanges, 0) +
      (taskGroups.mapBy('Tasks') || []).reduce(flatten, []).reduce(arrayOfFieldChanges, 0)
    );
  }

  @action
  toggleDiff() {
    this.toggleProperty('isOpen');
  }
}

const flatten = (accumulator, array) => accumulator.concat(array);
const countChanges = (total, field) => (changeTypes.includes(field.Type) ? total + 1 : total);

function fieldChanges(diff) {
  return (
    (diff.Fields || []).reduce(countChanges, 0) +
    (diff.Objects || []).reduce(arrayOfFieldChanges, 0)
  );
}

function arrayOfFieldChanges(count, diff) {
  if (!diff) {
    return count;
  }

  return count + fieldChanges(diff);
}
