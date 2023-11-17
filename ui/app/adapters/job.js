/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import WatchableNamespaceIDs from './watchable-namespace-ids';
import addToPath from 'nomad-ui/utils/add-to-path';
import { base64EncodeString } from 'nomad-ui/utils/encode';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { base64DecodeString } from '../utils/encode';
import config from 'nomad-ui/config/environment';

@classic
export default class JobAdapter extends WatchableNamespaceIDs {
  @service system;
  @service notifications;
  @service token;
  @service nomadActions;

  relationshipFallbackLinks = {
    summary: '/summary',
  };

  fetchRawDefinition(job) {
    const url = this.urlForFindRecord(job.get('id'), 'job');
    return this.ajax(url, 'GET');
  }

  fetchRawSpecification(job) {
    const url = addToPath(
      this.urlForFindRecord(job.get('id'), 'job', null, 'submission'),
      '',
      'version=' + job.get('version')
    );
    return this.ajax(url, 'GET');
  }

  forcePeriodic(job) {
    if (job.get('periodic')) {
      const url = addToPath(
        this.urlForFindRecord(job.get('id'), 'job'),
        '/periodic/force'
      );
      return this.ajax(url, 'POST');
    }
  }

  stop(job) {
    const url = this.urlForFindRecord(job.get('id'), 'job');
    return this.ajax(url, 'DELETE');
  }

  purge(job) {
    const url = addToPath(
      this.urlForFindRecord(job.get('id'), 'job'),
      '',
      'purge=true'
    );

    return this.ajax(url, 'DELETE');
  }

  parse(spec, jobVars) {
    const url = addToPath(this.urlForFindAll('job'), '/parse?namespace=*');
    return this.ajax(url, 'POST', {
      data: {
        JobHCL: spec,
        Variables: jobVars,
        Canonicalize: true,
      },
    });
  }

  plan(job) {
    const jobId = job.get('id') || job.get('_idBeforeSaving');
    const store = this.store;
    const url = addToPath(this.urlForFindRecord(jobId, 'job'), '/plan');

    return this.ajax(url, 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
        Diff: true,
      },
    }).then((json) => {
      json.ID = jobId;
      store.pushPayload('job-plan', { jobPlans: [json] });
      return store.peekRecord('job-plan', jobId);
    });
  }

  // Running a job doesn't follow REST create semantics so it's easier to
  // treat it as an action.
  run(job) {
    let Submission;
    try {
      JSON.parse(job.get('_newDefinition'));
      Submission = {
        Source: job.get('_newDefinition'),
        Format: 'json',
      };
    } catch {
      Submission = {
        Source: job.get('_newDefinition'),
        Format: 'hcl2',
        Variables: job.get('_newDefinitionVariables'),
      };
    }

    return this.ajax(this.urlForCreateRecord('job'), 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
        Submission,
      },
    });
  }

  update(job) {
    const jobId = job.get('id') || job.get('_idBeforeSaving');

    let Submission;
    try {
      JSON.parse(job.get('_newDefinition'));
      Submission = {
        Source: job.get('_newDefinition'),
        Format: 'json',
      };
    } catch {
      Submission = {
        Source: job.get('_newDefinition'),
        Format: 'hcl2',
        Variables: job.get('_newDefinitionVariables'),
      };
    }

    return this.ajax(this.urlForUpdateRecord(jobId, 'job'), 'POST', {
      data: {
        Job: job.get('_newDefinitionJSON'),
        Submission,
      },
    });
  }

  scale(job, group, count, message) {
    const url = addToPath(
      this.urlForFindRecord(job.get('id'), 'job'),
      '/scale'
    );
    return this.ajax(url, 'POST', {
      data: {
        Count: count,
        Message: message,
        Target: {
          Group: group,
        },
        Meta: {
          Source: 'nomad-ui',
        },
      },
    });
  }

  dispatch(job, meta, payload) {
    const url = addToPath(
      this.urlForFindRecord(job.get('id'), 'job'),
      '/dispatch'
    );
    return this.ajax(url, 'POST', {
      data: {
        Payload: base64EncodeString(payload),
        Meta: meta,
      },
    });
  }

  /**
   *
   * @param {import('../models/job').default} job
   * @param {import('../models/action').default} action
   * @param {string} allocID
   * @param {import('../models/action-instance').default} actionInstance
   * @returns {WebSocket}
   */
  runAction(job, action, allocID, actionInstance) {
    // let messageBuffer = '';

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const region = this.system.activeRegion;
    const applicationAdapter = getOwner(this).lookup('adapter:application');
    const prefix = `${
      applicationAdapter.host || window.location.host
    }/${applicationAdapter.urlPrefix()}`;

    const wsUrl =
      `${protocol}//${prefix}/job/${encodeURIComponent(
        job.get('name')
      )}/action` +
      `?namespace=${job.get('namespace.id')}&action=${
        action.name
      }&allocID=${allocID}&task=${action.task.name}&group=${
        action.task.taskGroup.name
      }&tty=true&ws_handshake=true` +
      (region ? `&region=${region}` : '');

    /**
     * @type {WebSocket}
     */
    let socket;

    const mirageEnabled =
      config.environment !== 'production' &&
      config['ember-cli-mirage'] &&
      config['ember-cli-mirage'].enabled !== false;

    if (mirageEnabled) {
      socket = new Object({
        messageDisplayed: false,
        addEventListener: function (event, callback) {
          if (event === 'message') {
            this.onmessage = callback;
          }
          if (event === 'open') {
            this.onopen = callback;
          }
          if (event === 'close') {
            this.onclose = callback;
          }
        },

        send(e) {
          if (!this.messageDisplayed) {
            this.messageDisplayed = true;
            this.onmessage({
              data: `{"stdout":{"data":"${btoa('Message Received')}"}}`,
            });
          } else {
            this.onmessage({ data: e.replace('stdin', 'stdout') });
          }
        },
      });
    } else {
      socket = new WebSocket(wsUrl);
    }

    actionInstance.set('socket', socket);

    // let notification;
    socket.addEventListener('open', () => {
      actionInstance.state = 'starting';
      actionInstance.createdAt = new Date();
      // notification = this.notifications
      //   .add({
      //     title: `Action ${action.name} Started`,
      //     color: 'neutral',
      //     code: true,
      //     sticky: true,
      //     customAction: {
      //       label: 'Stop Action',
      //       action: () => {
      //         socket.close();
      //       },
      //     },
      //   })
      //   .getFlashObject();

      socket.send(
        JSON.stringify({ version: 1, auth_token: this.token?.secret || '' })
      );
      socket.send(
        JSON.stringify({
          tty_size: { width: 250, height: 100 }, // Magic numbers, but they pass the eye test.
        })
      );
    });

    socket.addEventListener('message', (event) => {
      actionInstance.state = 'running';
      // TODO: Make sure we don't need to recreate socket close handling
      // if (!this.notifications.queue.includes(notification)) {
      //   // User has manually closed the notification;
      //   // explicitly close the socket and return;
      //   socket.close();
      //   return;
      // }

      let jsonData = JSON.parse(event.data);
      if (jsonData.stdout && jsonData.stdout.data) {
        // strip ansi escape characters that are common in action responses;
        // for example, we shouldn't show the newline or color code characters.
        // TODO: Don't process the whole .messages every time a new one comes in!
        actionInstance.messages += base64DecodeString(jsonData.stdout.data);
        actionInstance.messages += '\n';
        actionInstance.messages = actionInstance.messages.replace(
          /\x1b\[[0-9;]*[a-zA-Z]/g,
          ''
        );
        // notification.set('message', messageBuffer);
        // notification.set('title', `Action ${action.name} Running`);
      } else if (jsonData.stderr && jsonData.stderr.data) {
        // messageBuffer = base64DecodeString(jsonData.stderr.data);
        // messageBuffer += '\n';
        this.notifications.add({
          title: `Error received from ${action.name}`,
          // message: messageBuffer,
          color: 'critical',
          code: true,
          sticky: true,
        });
      }
    });

    socket.addEventListener('close', () => {
      actionInstance.state = 'complete';
      actionInstance.completedAt = new Date();
    });

    socket.addEventListener('error', function () {
      actionInstance.state = 'error';
      // TODO: implement instance.error
      // this.notifications.add({
      //   title: `Error received from ${action.name}`,
      //   message: event,
      //   color: 'critical',
      //   sticky: true,
      // });
    });

    if (mirageEnabled) {
      socket.onopen();
      socket.onclose();
    }

    return socket;
  }
}
