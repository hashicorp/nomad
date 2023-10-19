/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import WatchableNamespaceIDs from './watchable-namespace-ids';
import addToPath from 'nomad-ui/utils/add-to-path';
import { base64EncodeString } from 'nomad-ui/utils/encode';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { base64DecodeString } from '../utils/encode';

@classic
export default class JobAdapter extends WatchableNamespaceIDs {
  @service system;
  @service notifications;
  @service token;

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
    const url = this.urlForFindRecord(job.get('id'), 'job') + '?purge=true';
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

  runAction(job, action, allocID) {
    console.log('runAction from job adapter', job, action, allocID);

    let messageBuffer = '';

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const region = this.system.activeRegion;
    const applicationAdapter = getOwner(this).lookup('adapter:application');
    const prefix = `${
      applicationAdapter.host || window.location.host
    }/${applicationAdapter.urlPrefix()}`;

    const wsUrl =
      `${protocol}//${prefix}/job/${job.get('id')}/action` +
      `?namespace=*&action=${action.name}&allocID=${allocID}&task=${action.task.name}&group=${action.task.taskGroup.name}&tty=true&ws_handshake=true` +
      (region ? `&region=${region}` : '');

    const socket = new WebSocket(wsUrl);

    socket.addEventListener('open', function (event) {
      console.log('WebSocket connection opened:', event);
      socket.send(
        JSON.stringify({ version: 1, auth_token: this.token?.secret || '' })
      );
      socket.send(
        JSON.stringify({
          tty_size: { width: 250, height: 100 }, // TODO: verify that these are good defaults
        })
      );
    });

    socket.addEventListener('message', (event) => {
      console.log('WebSocket message received:', event);
      let jsonData = JSON.parse(event.data);
      console.log('jsonData', jsonData);
      if (jsonData.stdout && jsonData.stdout.data) {
        messageBuffer = base64DecodeString(jsonData.stdout.data);
        messageBuffer += '\n';
        this.notifications.add({
          title: `Action ${action.name} Message Received`,
          message: messageBuffer,
          color: 'success',
          code: true,
          sticky: true,
        });
      } else if (jsonData.stderr && jsonData.stderr.data) {
        messageBuffer = base64DecodeString(jsonData.stderr.data);
        messageBuffer += '\n';
        this.notifications.add({
          title: `Error received from ${action.name}`,
          message: messageBuffer,
          color: 'critical',
          code: true,
          sticky: true,
        });
      }
    });

    socket.addEventListener('close', (event) => {
      console.log('WebSocket connection closed:', event);
      this.notifications.add({
        title: `Action ${action.name} Completed`,
        message: messageBuffer || event.reason,
        color: 'success',
      });
      messageBuffer = '';
    });

    socket.addEventListener('error', function (event) {
      console.error('WebSocket encountered an error:', event);
    });

    return socket;
  }
}
