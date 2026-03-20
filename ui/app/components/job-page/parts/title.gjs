/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn, hash } from '@ember/helper';
import Component from '@glimmer/component';
import { htmlSafe } from '@ember/template';
import { not } from 'ember-truth-helpers';
import { service } from '@ember/service';
import { task } from 'ember-concurrency';
import can from 'ember-can/helpers/can';
import {
  HdsButton,
  HdsIcon,
  HdsPageHeader,
} from '@hashicorp/design-system-components/components';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import ActionsDropdown from 'nomad-ui/components/actions-dropdown';
import ExecOpenButton from 'nomad-ui/components/exec/open-button';
import TwoStepButton from 'nomad-ui/components/two-step-button';
import hdsTooltip from '@hashicorp/design-system-components/modifiers/hds-tooltip';
import keyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';
import jsonToHcl from 'nomad-ui/utils/json-to-hcl';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';

export default class Title extends Component {
  @service router;
  @service notifications;

  get displayTitle() {
    return this.args.title || this.args.job.name;
  }

  get hasPack() {
    return !!this.args.job.meta?.structured?.root.children.pack;
  }

  get showRunningActions() {
    return this.args.job.status !== 'dead';
  }

  get showActionsDropdown() {
    return this.args.job.actions.length && this.args.job.allocations.length;
  }

  get showStableVersionRevert() {
    return this.args.job.hasStableNonCurrentVersion;
  }

  get showLatestVersionRevert() {
    return !this.args.job.hasVersionStability && this.args.job.latestVersion;
  }

  stopJob = task(async (withNotifications = false) => {
    try {
      const job = this.args.job;
      await job.stop();
      job.set('status', 'dead');

      if (withNotifications) {
        this.notifications.add({
          title: 'Job Stopped',
          message: `${job.name} has been stopped`,
          color: 'success',
        });
      }
    } catch (err) {
      this.args.handleError?.({
        title: 'Could Not Stop Job',
        description: messageFromAdapterError(err, 'stop jobs'),
      });
    }
  });

  purgeJob = task(async () => {
    try {
      const job = this.args.job;
      await job.purge();
      this.notifications.add({
        title: 'Job Purged',
        message: `You have purged ${job.name}`,
        color: 'success',
      });
      this.router.transitionTo('jobs');
    } catch (err) {
      this.args.handleError?.({
        title: 'Error purging job',
        description: messageFromAdapterError(err, 'purge jobs'),
      });
    }
  });

  startJob = task(async (withNotifications = false) => {
    const job = this.args.job;

    try {
      const specification = await job.fetchRawSpecification();

      let newDefinitionVariables = job.get('_newDefinitionVariables') || '';

      if (specification.VariableFlags) {
        newDefinitionVariables += jsonToHcl(specification.VariableFlags);
      }

      if (specification.Variables) {
        newDefinitionVariables += specification.Variables;
      }

      job.set('_newDefinitionVariables', newDefinitionVariables);
      job.set('_newDefinition', specification.Source);
    } catch {
      const definition = await job.fetchRawDefinition();
      delete definition.Stop;
      job.set('_newDefinition', JSON.stringify(definition));
    }

    try {
      await job.parse();
      await job.update();
      job.set('status', 'running');

      if (withNotifications) {
        this.notifications.add({
          title: 'Job Started',
          message: `${job.name} has started`,
          color: 'success',
        });
      }
    } catch (err) {
      this.args.handleError?.({
        title: 'Could Not Start Job',
        description: messageFromAdapterError(err, 'start jobs'),
      });
    }
  });

  revertTo = task(async (version) => {
    if (!version) {
      return;
    }

    await version.revertTo();
  });

  get description() {
    if (!this.args.job.ui?.Description) {
      return null;
    }

    marked.use({
      gfm: true,
      breaks: true,
    });

    const purifyConfig = {
      FORBID_TAGS: ['script', 'style'],
      FORBID_ATTR: ['onerror', 'onload'],
    };
    const rawDescription = marked.parse(this.args.job.ui.Description);

    if (typeof rawDescription !== 'string') {
      console.error(
        'Expected a string from marked.parse(), received:',
        typeof rawDescription,
      );
      return null;
    }

    const cleanDescription = DOMPurify.sanitize(rawDescription, purifyConfig);
    return htmlSafe(cleanDescription);
  }

  get links() {
    return this.args.job.ui?.Links;
  }

  <template>
    <HdsPageHeader class="job-page-header" as |PH|>
      <PH.Title data-test-job-name>
        {{this.displayTitle}}
        {{#if this.hasPack}}
          <span data-test-pack-tag class="tag is-hollow">
            <HdsIcon @name="box" @color="faint" />
            <span>Pack</span>
          </span>
        {{/if}}
        {{yield}}
      </PH.Title>
      {{#if this.description}}
        <PH.Description data-test-job-description>
          {{this.description}}
        </PH.Description>
      {{/if}}
      {{#if this.links}}
        <PH.Generic>
          <div class="job-ui-links" data-test-job-links>
            {{#each this.links as |link|}}
              <HdsButton
                @color="secondary"
                @isInline={{true}}
                @text={{link.Label}}
                @icon="external-link"
                @iconPosition="trailing"
                @href={{link.Url}}
              />
            {{/each}}
          </div>
        </PH.Generic>
      {{/if}}
      <PH.Actions>
        {{#if this.showRunningActions}}
          {{#if (can "exec allocation" namespace=@job.namespaceId)}}
            {{#if this.showActionsDropdown}}
              <ActionsDropdown @actions={{@job.actions}} />
            {{/if}}
          {{/if}}
          <ExecOpenButton @job={{@job}} />
          <TwoStepButton
            data-test-stop
            @alignRight={{true}}
            @idleText="Stop Job"
            @disabled={{not (can "stop job" namespace=@job.namespaceId)}}
            @title={{if
              (can "stop job" namespace=@job.namespaceId)
              null
              "You don’t have permission to stop jobs"
            }}
            @cancelText="Cancel"
            @confirmText="Yes, Stop Job"
            @confirmationMessage="Are you sure you want to stop this job?"
            @awaitingConfirmation={{this.stopJob.isRunning}}
            @onConfirm={{this.stopJob.perform}}
            {{keyboardShortcutModifier
              label="Stop"
              pattern=(array "s" "t" "o" "p")
              action=(fn this.stopJob.perform true)
            }}
          />
        {{else}}
          <TwoStepButton
            data-test-purge
            @alignRight={{true}}
            @idleText="Purge Job"
            @disabled={{not (can "purge job" namespace=@job.namespaceId)}}
            @title={{if
              (can "purge job" namespace=@job.namespaceId)
              null
              "You don’t have permission to purge jobs"
            }}
            @cancelText="Cancel"
            @confirmText="Yes, Purge Job"
            @confirmationMessage="Are you sure? You cannot undo this action."
            @awaitingConfirmation={{this.purgeJob.isRunning}}
            @onConfirm={{this.purgeJob.perform}}
            {{keyboardShortcutModifier
              label="Purge"
              pattern=(array "p" "u" "r" "g" "e")
              action=this.purgeJob.perform
            }}
          />

          {{#if @job.stopped}}
            <TwoStepButton
              data-test-start
              @alignRight={{true}}
              @idleText="Start Job"
              @disabled={{not (can "start job" namespace=@job.namespaceId)}}
              @title={{if
                (can "start job" namespace=@job.namespaceId)
                null
                "You don’t have permission to start jobs"
              }}
              @cancelText="Cancel"
              @confirmText="Yes, Start Job"
              @confirmationMessage="Are you sure you want to start this job?"
              @awaitingConfirmation={{this.startJob.isRunning}}
              @onConfirm={{this.startJob.perform}}
              {{keyboardShortcutModifier
                label="Start"
                pattern=(array "s" "t" "a" "r" "t")
                action=(fn this.startJob.perform true)
              }}
            />
          {{else if this.showStableVersionRevert}}
            <TwoStepButton
              data-test-revert
              @alignRight={{true}}
              @idleText="Revert to last stable version (v{{@job.latestStableVersion.number}})"
              @disabled={{not (can "revert job" namespace=@job.namespaceId)}}
              @title={{if
                (can "revert job" namespace=@job.namespaceId)
                null
                "You don’t have permission to revert jobs"
              }}
              @cancelText="Cancel"
              @confirmText="Yes, Revert to last stable version"
              @confirmationMessage="Are you sure you want to revert to the last stable version?"
              @awaitingConfirmation={{this.revertTo.isRunning}}
              @onConfirm={{fn this.revertTo.perform @job.latestStableVersion}}
            />
          {{else if this.showLatestVersionRevert}}
            <TwoStepButton
              data-test-revert
              @alignRight={{true}}
              @idleText="Revert to last version (v{{@job.latestVersion.number}})"
              @cancelText="Cancel"
              @confirmText="Yes, Revert to last version"
              @confirmationMessage="Are you sure you want to revert to the last version?"
              @awaitingConfirmation={{this.revertTo.isRunning}}
              @onConfirm={{fn this.revertTo.perform @job.latestVersion}}
            />
            <HdsButton
              data-test-edit-and-resubmit
              @color="primary"
              @isInline={{true}}
              @text="Edit and Resubmit job"
              @route="jobs.job.definition"
              @query={{hash isEditing=true}}
            />
          {{else}}
            <HdsButton
              data-test-edit-and-resubmit
              {{hdsTooltip
                "This job has failed and has no stable previous version to fall back to. You can edit and resubmit the job to try again."
                options=(hash placement="bottom")
              }}
              @color="primary"
              @isInline={{true}}
              @text="Edit and Resubmit job"
              @route="jobs.job.definition"
              @query={{hash isEditing=true}}
            />
          {{/if}}
        {{/if}}
      </PH.Actions>
    </HdsPageHeader>
  </template>
}
