/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';

const changeTypes = ['Added', 'Deleted', 'Edited'];

export default class JobVersion extends Component {
  @service store;
  @service notifications;
  @service router;

  @alias('args.version') version;
  @alias('args.diff') diff;
  @tracked isOpen = false;
  @tracked isEditing = false;
  @tracked editableTag;

  // Passes through to the job-diff component
  verbose = true;

  constructor() {
    super(...arguments);
    this.initializeEditableTag();
    this.versionsDidUpdate();
  }

  @action versionsDidUpdate() {
    if (this.args.diffsExpanded && this.diff) {
      this.isOpen = true;
    }
  }

  initializeEditableTag() {
    if (this.version.versionTag) {
      this.editableTag = this.store.createRecord('versionTag', {
        name: this.version.versionTag.name,
        description: this.version.versionTag.description,
      });
    } else {
      this.editableTag = this.store.createRecord('versionTag');
    }
    this.editableTag.versionNumber = this.version.number;
    this.editableTag.jobName = this.version.get('job.plainId');
  }

  @computed('diff')
  get changeCount() {
    const diff = this.diff;
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

  /**
   * @type {'idle' | 'confirming'}
   */
  @tracked cloneButtonStatus = 'idle';

  @task(function* () {
    try {
      const versionBeforeReversion = this.version.get('job.version');
      yield this.version.revertTo();
      yield this.version.get('job').reload();

      const versionAfterReversion = this.version.get('job.version');
      if (versionBeforeReversion === versionAfterReversion) {
        // TODO: I don't think this is ever hit, we have template checks against it.
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
      this.args.handleError({
        level: 'danger',
        title: 'Could Not Revert',
        description: messageForError(e, 'revert'),
      });
    }
  })
  revertTo;

  @action async cloneAsNewVersion() {
    try {
      this.router.transitionTo(
        'jobs.job.definition',
        this.version.get('job.idWithNamespace'),
        {
          queryParams: {
            isEditing: true,
            version: this.version.number,
          },
        }
      );
    } catch (e) {
      this.args.handleError({
        level: 'danger',
        title: 'Could Not Edit from Version',
      });
    }
  }

  @action async cloneAsNewJob() {
    console.log('cloneAsNewJob');
    try {
      // TODO: copy the job definition over there.
      console.log('Do I have submission info???', this.version);
      let job = await this.version.get('job');
      let specification = await job.fetchRawSpecification(this.version.number);
      console.log('Do I have specification???', specification);
      let specificationSourceString = specification.Source; // TODO: should do some Format checking here, at the very least.
      this.router.transitionTo('jobs.run', {
        queryParams: {
          sourceString: specificationSourceString,
        },
      });
    } catch (e) {
      this.args.handleError({
        level: 'danger',
        title: 'Could Not Clone as New Job',
      });
    }
  }

  @action
  handleKeydown(event) {
    if (event.key === 'Escape') {
      this.cancelEditTag();
    }
  }

  @action
  toggleEditTag() {
    this.isEditing = !this.isEditing;
  }

  @action
  async saveTag(event) {
    event.preventDefault();
    try {
      if (!this.editableTag.name) {
        this.notifications.add({
          title: 'Error Tagging Job Version',
          message: 'Tag name is required',
          color: 'critical',
        });
        return;
      }
      const savedTag = await this.editableTag.save();
      this.version.versionTag = savedTag;
      this.version.versionTag.setProperties({
        ...savedTag.toJSON(),
      });
      this.initializeEditableTag();
      this.isEditing = false;

      this.notifications.add({
        title: 'Job Version Tagged',
        color: 'success',
      });
    } catch (error) {
      console.log('error tagging job version', error);
      this.notifications.add({
        title: 'Error Tagging Job Version',
        message: messageForError(error),
        color: 'critical',
      });
    }
  }

  @action
  cancelEditTag() {
    this.isEditing = false;
    this.initializeEditableTag();
  }

  @action
  async deleteTag() {
    try {
      await this.store
        .adapterFor('version-tag')
        .deleteTag(this.editableTag.jobName, this.editableTag.name);
      this.notifications.add({
        title: 'Job Version Un-Tagged',
        color: 'success',
      });
      this.version.versionTag = null;
      this.initializeEditableTag();
      this.isEditing = false;
    } catch (error) {
      this.notifications.add({
        title: 'Error Un-Tagging Job Version',
        message: messageForError(error),
        color: 'critical',
      });
    }
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
