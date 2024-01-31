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
import { get } from '@ember/object';
import queryString from 'query-string';

@classic
export default class JobAdapter extends WatchableNamespaceIDs {
  @service system;
  @service watchList;

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

  query(store, type, query, snapshotRecordArray, options) {
    options = options || {};
    options.adapterOptions = options.adapterOptions || {};

    const method = get(options, 'adapterOptions.method') || 'GET';
    const url = this.urlForQuery(query, type.modelName, method);

    // Let's establish the index, via watchList.getIndexFor.
    let index = this.watchList.getIndexFor(url);

    // In the case that this is of queryType update,
    // and its index is found to be 1,
    // check for the initialize query's index and use that instead
    // if (options.adapterOptions.queryType === 'update' && index === 1) {
    //   let initializeQueryIndex = this.watchList.getIndexFor(
    //     '/v1/jobs/statuses3?meta=true&per_page=10'
    //   ); // TODO: fickle!
    //   if (initializeQueryIndex) {
    //     index = initializeQueryIndex;
    //   }
    // }

    // Disregard the index if this is an initialize query
    if (options.adapterOptions.queryType === 'initialize') {
      index = null;
    }

    // TODO: adding a new job hash will not necessarily cancel the old one.
    // You could be holding open a POST on jobs AB and ABC at the same time.

    console.log('index for', url, 'is', index);

    if (index && index > 1) {
      query.index = index;
    }

    const signal = get(options, 'adapterOptions.abortController.signal');

    return this.ajax(url, method, {
      signal,
      data: query,
      skipURLModification: true,
    }).then((payload) => {
      console.log('thenner', payload, query);
      // If there was a request body, append it to my payload
      if (query.jobs) {
        payload._requestBody = query;
      }
      return payload;
    });
  }

  handleResponse(status, headers, payload, requestData) {
    // watchList.setIndexFor() happens in the watchable adapter, super'd here

    /**
     * @type {Object}
     */
    const result = super.handleResponse(...arguments);
    if (result) {
      result.meta = result.meta || {};
      if (headers['x-nomad-nexttoken']) {
        result.meta.nextToken = headers['x-nomad-nexttoken'];
      }
    }

    return result;
  }

  urlForQuery(query, modelName, method) {
    let baseUrl = `/${this.namespace}/jobs/statuses3`;
    if (method === 'POST') {
      // Setting a base64 hash to represent the body of the POST request
      // (which is otherwise not represented in the URL)
      // because the watchList uses the URL as a key for index lookups.
      return `${baseUrl}?hash=${btoa(JSON.stringify(query))}`;
    } else {
      return `${baseUrl}?${queryString.stringify(query)}`;
    }
  }

  ajaxOptions(url, type, options) {
    let hash = super.ajaxOptions(url, type, options);
    // Custom handling for POST requests to append 'index' as a query parameter
    if (type === 'POST' && options.data && options.data.index) {
      let index = encodeURIComponent(options.data.index);
      hash.url = `${hash.url}&index=${index}`;
    }

    return hash;
  }
}
