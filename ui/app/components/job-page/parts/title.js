/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@ember/component';
import { task } from 'ember-concurrency';
import { inject as service } from '@ember/service';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import { action } from '@ember/object';

@classic
@tagName('')
export default class Title extends Component {
  @service router;
  @service notifications;
  /**
   * @type { import('../../../services/nomad-actions') }
   */
  @service nomadActions;

  job = null;
  title = null;

  handleError() {}

  /**
   * @param {boolean} withNotifications - Whether to show a toast on success, as when triggered by keyboard shortcut
   */
  @task(function* (withNotifications = false) {
    try {
      const job = this.job;
      yield job.stop();
      // Eagerly update the job status to avoid flickering
      job.set('status', 'dead');
      if (withNotifications) {
        this.notifications.add({
          title: 'Job Stopped',
          message: `${job.name} has been stopped`,
          color: 'success',
        });
      }
    } catch (err) {
      this.handleError({
        title: 'Could Not Stop Job',
        description: messageFromAdapterError(err, 'stop jobs'),
      });
    }
  })
  stopJob;

  @task(function* () {
    try {
      const job = this.job;
      yield job.purge();
      this.notifications.add({
        title: 'Job Purged',
        message: `You have purged ${job.name}`,
        color: 'success',
      });
      this.router.transitionTo('jobs');
    } catch (err) {
      this.handleError({
        title: 'Error purging job',
        description: messageFromAdapterError(err, 'purge jobs'),
      });
    }
  })
  purgeJob;

  /**
   * @param {boolean} withNotifications - Whether to show a toast on success, as when triggered by keyboard shortcut
   */
  @task(function* (withNotifications = false) {
    const job = this.job;

    // Try to get the submission/hcl sourced specification first.
    // In the event that this fails, fall back to the raw definition.
    try {
      const specification = yield job.fetchRawSpecification();
      job.set('_newDefinition', specification.Source);
    } catch {
      const definition = yield job.fetchRawDefinition();
      delete definition.Stop;
      job.set('_newDefinition', JSON.stringify(definition));
    }

    try {
      yield job.parse();
      yield job.update();
      // Eagerly update the job status to avoid flickering
      job.set('status', 'running');
      if (withNotifications) {
        this.notifications.add({
          title: 'Job Started',
          message: `${job.name} has started`,
          color: 'success',
        });
      }
    } catch (err) {
      this.handleError({
        title: 'Could Not Start Job',
        description: messageFromAdapterError(err, 'start jobs'),
      });
    }
  })
  startJob;

  // run action task

  /**
   * @param {string} action - The action to run
   * @param {string} allocID - The allocation ID to run the action on
   * @param {Event} ev - The event that triggered the action
   */
  @task(function* (action, allocID) {
    if (!allocID) {
      allocID =
        action.allocations[
          Math.floor(Math.random() * action.allocations.length)
        ].id;
    }
    try {
      const job = this.job;
      // TODO: have the service handle "all" vs specific
      if (allocID === 'all') {
        yield action.allocations.map((alloc) => {
          this.nomadActions.runAction(action, alloc.id, job);
        });
      } else {
        this.nomadActions.runAction(action, allocID, job);
      }
    } catch (err) {
      this.notifications.add({
        title: `Error starting ${action.name}`,
        message: err,
        sticky: true,
        color: 'critical',
      });
    }
  })
  runAction;

  /**
   * @param {HTMLElement} el
   */
  @action openActionsDropdown(el) {
    const dropdownTrigger = el?.getElementsByTagName('button')[0];
    if (dropdownTrigger) {
      dropdownTrigger.click();
    }
  }
}
