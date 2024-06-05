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
import jsonToHcl from 'nomad-ui/utils/json-to-hcl';
import { marked } from 'marked';
import { htmlSafe } from '@ember/template';
import DOMPurify from 'dompurify';

@classic
@tagName('')
export default class Title extends Component {
  @service router;
  @service notifications;

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

      let _newDefinitionVariables = job.get('_newDefinitionVariables') || '';
      if (specification.VariableFlags) {
        _newDefinitionVariables += jsonToHcl(specification.VariableFlags);
      }
      if (specification.Variables) {
        _newDefinitionVariables += specification.Variables;
      }
      job.set('_newDefinitionVariables', _newDefinitionVariables);

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

  get description() {
    if (!this.job.ui?.Description) {
      return null;
    }

    // Put <br /> on newlines, use github-flavoured-markdown.
    marked.use({
      gfm: true,
      breaks: true,
    });

    const purifyConfig = {
      FORBID_TAGS: ['script', 'style'],
      FORBID_ATTR: ['onerror', 'onload'],
    };
    const rawDescription = marked.parse(this.job.ui.Description);
    if (typeof rawDescription !== 'string') {
      console.error(
        'Expected a string from marked.parse(), received:',
        typeof rawDescription
      );
      return null;
    }
    const cleanDescription = DOMPurify.sanitize(rawDescription, purifyConfig);
    return htmlSafe(cleanDescription);
  }

  get links() {
    return this.job.ui?.Links;
  }
}
