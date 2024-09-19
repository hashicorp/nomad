/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';

const changeTypes = ['Added', 'Deleted', 'Edited'];

export default class JobVersion extends Component {
  @alias('args.version') version;
  @tracked isOpen = false;
  @tracked isEditing = false;

  // Passes through to the job-diff component
  verbose = true;

  @service router;

  @computed('version.diff')
  get changeCount() {
    const diff = this.version.diff;
    const taskGroups = diff.TaskGroups || [];

    if (!diff) {
      return 0;
    }

    return (
      fieldChanges(diff) +
      taskGroups.reduce(arrayOfFieldChanges, 0) +
      (taskGroups.mapBy('Tasks') || [])
        .reduce(flatten, [])
        .reduce(arrayOfFieldChanges, 0)
    );
  }

  @computed('version.{number,job.version}')
  get isCurrent() {
    return this.version.number === this.version.get('job.version');
  }

  @action
  toggleDiff() {
    this.isOpen = !this.isOpen;
  }

  @task(function* () {
    try {
      const versionBeforeReversion = this.version.get('job.version');
      yield this.version.revertTo();
      yield this.version.get('job').reload();

      const versionAfterReversion = this.version.get('job.version');
      if (versionBeforeReversion === versionAfterReversion) {
        this.args.handleError({
          level: 'warn',
          title: 'Reversion Had No Effect',
          description:
            'Reverting to an identical older version doesn’t produce a new version',
        });
      } else {
        const job = this.version.get('job');
        this.router.transitionTo('jobs.job.index', job.get('idWithNamespace'));
      }
    } catch (e) {
      console.log('catchy', e);
      this.args.handleError({
        level: 'danger',
        title: 'Could Not Revert',
        description: messageForError(e, 'revert'),
      });
    }
  })
  revertTo;

  @action
  toggleEditTag() {
    this.isEditing = !this.isEditing;
  }

  @action
  saveTag() {
    this.isEditing = false;
    this.editableTag.save();
  }

  @action
  cancelEditTag() {
    this.isEditing = false;
  }

  get editableTag() {
    return this.version.taggedVersion || {};
  }
}

const flatten = (accumulator, array) => accumulator.concat(array);
const countChanges = (total, field) =>
  changeTypes.includes(field.Type) ? total + 1 : total;

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
