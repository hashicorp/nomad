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

@classic
export default class JobAdapter extends WatchableNamespaceIDs {
  @service system;

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
   * @returns {string}
   */
  getActionSocketUrl(job, action, allocID) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const region = this.system.activeRegion;

    /**
     * @type {Partial<import('../adapters/application').default>}
     */
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
      }&allocID=${allocID}&task=${
        action.task.name
      }&tty=true&ws_handshake=true` +
      (region ? `&region=${region}` : '');

    return wsUrl;
  }
}
