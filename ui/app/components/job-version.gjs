/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { didUpdate } from '@ember/render-modifiers';
import { service } from '@ember/service';
import { task } from 'ember-concurrency';
import can from 'ember-can/helpers/can';
import { eq, not } from 'ember-truth-helpers';
import {
  HdsButton,
  HdsFormTextInputField,
} from '@hashicorp/design-system-components/components';
import JobDiff from 'nomad-ui/components/job-diff';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import formatTs from 'nomad-ui/helpers/format-ts';
import pluralize from 'nomad-ui/helpers/pluralize';
import messageForError from 'nomad-ui/utils/message-from-adapter-error';

const changeTypes = ['Added', 'Deleted', 'Edited'];

export default class JobVersion extends Component {
  @service store;
  @service notifications;
  @service router;

  @tracked isOpen = false;
  @tracked isEditing = false;
  @tracked editableTag;
  @tracked cloneButtonStatus = 'idle';

  verbose = true;

  constructor() {
    super(...arguments);
    this.isOpen = Boolean(this.args.diffsExpanded && this.diff);
  }

  get version() {
    return this.args.version;
  }

  get diff() {
    return this.args.diff;
  }

  versionsDidUpdate = () => {
    if (this.args.diffsExpanded && this.diff) {
      this.isOpen = true;
    }
  };

  initializeEditableTag() {
    const job = this.version.get('job');
    const namespaceId =
      this.version.get('job.namespaceId') ||
      job.belongsTo('namespace').id() ||
      'default';
    const jobName = this.version.get('job.plainId');

    if (this.version.versionTag) {
      this.editableTag = this.store.createRecord('versionTag', {
        name: this.version.versionTag.name,
        description: this.version.versionTag.description,
      });
    } else {
      this.editableTag = this.store.createRecord('versionTag');
    }
    this.editableTag.versionNumber = this.version.number;
    this.editableTag.jobNamespace = namespaceId;
    this.editableTag.jobName = jobName;
  }

  get changeCount() {
    const diff = this.diff;
    const taskGroups = diff?.TaskGroups || [];

    if (!diff) {
      return 0;
    }

    return (
      fieldChanges(diff) +
      taskGroups.reduce(arrayOfFieldChanges, 0) +
      (taskGroups.map((taskGroup) => taskGroup?.Tasks) || [])
        .reduce(flatten, [])
        .reduce(arrayOfFieldChanges, 0)
    );
  }

  get isCurrent() {
    return this.version.number === this.version.get('job.version');
  }

  toggleDiff = () => {
    this.isOpen = !this.isOpen;
  };

  revertTo = task(async () => {
    try {
      const versionBeforeReversion = this.version.get('job.version');
      await this.version.revertTo();
      await this.version.get('job').reload();

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
    } catch (error) {
      this.args.handleError({
        level: 'danger',
        title: 'Could Not Revert',
        description: messageForError(error, 'revert'),
      });
    }
  });

  performRevert = () => {
    this.revertTo.perform();
  };

  cloneAsNewVersion = async () => {
    try {
      this.router.transitionTo(
        'jobs.job.definition',
        this.version.get('job.idWithNamespace'),
        {
          queryParams: {
            isEditing: true,
            version: this.version.number,
          },
        },
      );
    } catch {
      this.args.handleError({
        level: 'danger',
        title: 'Could Not Edit from Version',
      });
    }
  };

  cloneAsNewJob = async () => {
    const job = await this.version.get('job');
    try {
      const specification = await job.fetchRawSpecification(
        this.version.number,
      );
      this.router.transitionTo('jobs.run', {
        queryParams: {
          sourceString: specification.Source,
        },
      });
      return;
    } catch {
      try {
        const definition = await job.fetchRawDefinition(this.version.number);
        this.router.transitionTo('jobs.run', {
          queryParams: {
            sourceString: JSON.stringify(definition, null, 2),
          },
        });
      } catch (definitionError) {
        this.args.handleError({
          level: 'danger',
          title: 'Could Not Clone as New Job',
          description: messageForError(definitionError),
        });
      }
    }
  };

  handleKeydown = (event) => {
    if (event.key === 'Escape') {
      this.cancelEditTag();
    }
  };

  updateEditableTagName = ({ target: { value } }) => {
    this.editableTag.name = value;
  };

  updateEditableTagDescription = ({ target: { value } }) => {
    this.editableTag.description = value;
  };

  toggleEditTag = () => {
    if (!this.isEditing) {
      this.initializeEditableTag();
    }

    this.isEditing = !this.isEditing;
  };

  saveTag = async (event) => {
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
      const effectiveTag = savedTag || this.editableTag;
      const tagData =
        typeof effectiveTag.toJSON === 'function'
          ? effectiveTag.toJSON()
          : {
              name: this.editableTag.name,
              description: this.editableTag.description,
              versionNumber: this.editableTag.versionNumber,
            };

      this.version.versionTag = effectiveTag;
      if (typeof this.version.versionTag?.setProperties === 'function') {
        this.version.versionTag.setProperties({
          ...tagData,
        });
      }

      this.initializeEditableTag();
      this.isEditing = false;

      this.notifications.add({
        title: 'Job Version Tagged',
        color: 'success',
      });
    } catch (error) {
      this.notifications.add({
        title: 'Error Tagging Job Version',
        message: messageForError(error),
        color: 'critical',
      });
    }
  };

  cancelEditTag = () => {
    this.isEditing = false;
    this.initializeEditableTag();
  };

  deleteTag = async () => {
    try {
      await this.store
        .adapterFor('version-tag')
        .deleteTag(
          this.editableTag.jobNamespace,
          this.editableTag.jobName,
          this.editableTag.name,
        );
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
  };

  <template>
    <span hidden {{didUpdate this.versionsDidUpdate this.diff}}></span>
    <section class="job-version" data-test-job-version={{this.version.number}}>
      <div
        class="boxed-section {{if this.version.versionTag 'tagged'}}"
        data-test-tagged-version={{if this.version.versionTag "true" "false"}}
      >
        <header class="boxed-section-head is-light inline-definitions">
          Version #{{this.version.number}}

          {{#if this.version.job.hasVersionStability}}
            <span class="bumper-left pair is-faded">
              <span class="term">Stable</span>
              <span
                class="badge is-light is-faded"
                data-test-version-stability
              ><code>{{this.version.stable}}</code></span>
            </span>
          {{else}}
            <span class="bumper-left" />
          {{/if}}
          <span class="pair is-faded">
            <span class="term">Submitted</span>
            <span data-test-version-submit-time class="submit-date">{{formatTs
                this.version.submitTime
              }}</span>
          </span>
          <div class="pull-right">
            {{#if this.diff}}
              <HdsButton
                class="is-light is-small"
                @size="small"
                @text="{{if
                  this.isOpen
                  'Hide'
                  'See'
                }} {{this.changeCount}} {{pluralize 'Change' this.changeCount}}"
                @color="tertiary"
                @icon={{if this.isOpen "chevron-up" "chevron-down"}}
                @iconPosition="trailing"
                {{on "click" this.toggleDiff}}
              />
            {{else}}
              <div class="is-fixed-width is-size-7 has-text-centered">No Changes</div>
            {{/if}}
          </div>
        </header>
        {{#if this.isOpen}}
          <div class="boxed-section-body is-dark">
            <JobDiff @diff={{this.diff}} @verbose={{this.verbose}} />
          </div>
        {{/if}}
        <footer class="boxed-section-foot {{if this.isEditing 'editing'}}">
          {{#if this.isEditing}}
            <form
              class="tag-options"
              autocomplete="off"
              {{on "submit" this.saveTag}}
            >
              <HdsFormTextInputField
                data-test-tag-name-input
                @label="Tag Name"
                placeholder="Tag Name"
                @value={{this.editableTag.name}}
                @size="small"
                {{on "input" this.updateEditableTagName}}
                {{on "keydown" this.handleKeydown}}
              />
              <HdsFormTextInputField
                data-test-tag-description-input
                @label="Tag Description"
                placeholder="Tag Description"
                @value={{this.editableTag.description}}
                {{on "input" this.updateEditableTagDescription}}
                {{on "keydown" this.handleKeydown}}
              />
              <HdsButton
                data-test-tag-save-button
                type="submit"
                @text="Save"
                @color="primary"
                @size="small"
                @isInline={{true}}
                {{on "click" this.saveTag}}
              />
              <HdsButton
                @text="Cancel"
                @color="secondary"
                @size="small"
                @isInline={{true}}
                {{on "click" this.cancelEditTag}}
              />
              {{#if this.version.versionTag}}
                <HdsButton
                  data-test-tag-delete-button
                  @text="Delete"
                  @color="critical"
                  @size="small"
                  @isInline={{true}}
                  {{on "click" this.deleteTag}}
                />
              {{/if}}
            </form>
          {{else}}
            <div class="tag-options">
              {{#if this.version.versionTag}}
                <HdsButton
                  class="tag-button-primary"
                  @text={{this.version.versionTag.name}}
                  @color="primary"
                  @size="small"
                  @icon="tag"
                  @iconPosition="leading"
                  @isInline={{true}}
                  {{on "click" this.toggleEditTag}}
                />
              {{else}}
                {{#if (can "tag version" namespace=this.version.job.namespace)}}
                  <HdsButton
                    data-test-version-tag
                    class="tag-button-secondary"
                    @text="Tag this version"
                    @color="secondary"
                    @size="small"
                    @icon="tag"
                    @iconPosition="leading"
                    @isInline={{true}}
                    {{on "click" this.toggleEditTag}}
                  />
                {{/if}}
              {{/if}}
              <span
                class="tag-description"
                title={{this.version.versionTag.description}}
              >
                {{this.version.versionTag.description}}
              </span>
            </div>
            <div class="version-options">
              {{#unless this.isCurrent}}
                {{#if (eq this.cloneButtonStatus "idle")}}
                  {{#if (can "run job")}}
                    <HdsButton
                      data-test-clone-and-edit
                      @text="Clone and Edit"
                      @color="secondary"
                      @size="small"
                      @isInline={{true}}
                      {{on
                        "click"
                        (fn (mut this.cloneButtonStatus) "confirming")
                      }}
                    />
                  {{/if}}
                  <TwoStepButton
                    data-test-revert-to
                    @classes={{hash
                      idleButton="is-warning is-outlined"
                      confirmButton="is-warning"
                    }}
                    @fadingBackground={{true}}
                    @idleText="Revert Version"
                    @disabled={{not
                      (can "revert job" namespace=this.version.job.namespaceId)
                    }}
                    @title={{if
                      (can "revert job" namespace=this.version.job.namespaceId)
                      null
                      "You don’t have permission to revert"
                    }}
                    @cancelText="Cancel"
                    @confirmText="Yes, Revert Version"
                    @confirmationMessage="Are you sure you want to revert to this version?"
                    @inlineText={{true}}
                    @size="small"
                    @awaitingConfirmation={{this.revertTo.isRunning}}
                    @onConfirm={{this.performRevert}}
                  />
                {{else if (eq this.cloneButtonStatus "confirming")}}
                  <HdsButton
                    data-test-cancel-clone
                    @text="Cancel"
                    @color="secondary"
                    @size="small"
                    @isInline={{true}}
                    {{on "click" (fn (mut this.cloneButtonStatus) "idle")}}
                  />
                  {{#if
                    (can "start job" namespace=this.version.job.namespaceId)
                  }}
                    <HdsButton
                      data-test-clone-as-new-version
                      @text="Clone as New Version of {{this.version.job.name}}"
                      @color="secondary"
                      @size="small"
                      @isInline={{true}}
                      {{on "click" this.cloneAsNewVersion}}
                    />
                  {{/if}}
                  <HdsButton
                    data-test-clone-as-new-job
                    @text="Clone as New Job"
                    @color="secondary"
                    @size="small"
                    @isInline={{true}}
                    {{on "click" this.cloneAsNewJob}}
                  />
                {{/if}}
              {{/unless}}
            </div>
          {{/if}}
        </footer>
      </div>
    </section>
  </template>
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
